package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
	"sync"
	"time"
)

const Exchange = "cowhealth.v1"

func Connect() (*amqp.Connection, *amqp.Channel, error) {
	c, e := amqp.DialConfig(Required("AMQP_URL"), amqp.Config{Heartbeat: 10 * time.Second, Dial: amqp.DefaultDial(5 * time.Second)})
	if e != nil {
		return nil, nil, e
	}
	ch, e := c.Channel()
	if e != nil {
		c.Close()
		return nil, nil, e
	}
	if e = ch.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); e != nil {
		c.Close()
		return nil, nil, e
	}
	if e = ch.ExchangeDeclare("cowhealth.dlx", "fanout", true, false, false, false, nil); e != nil {
		c.Close()
		return nil, nil, e
	}
	if _, e = ch.QueueDeclare("telemetry.dead", true, false, false, false, nil); e != nil {
		c.Close()
		return nil, nil, e
	}
	if e = ch.QueueBind("telemetry.dead", "", "cowhealth.dlx", false, nil); e != nil {
		c.Close()
		return nil, nil, e
	}
	if _, e = ch.QueueDeclare("telemetry.received.v1", true, false, false, false, amqp.Table{"x-dead-letter-exchange": "cowhealth.dlx", "x-queue-type": "quorum", "x-delivery-limit": 5}); e != nil {
		c.Close()
		return nil, nil, e
	}
	if e = ch.QueueBind("telemetry.received.v1", "telemetry.received.v1", Exchange, false, nil); e != nil {
		c.Close()
		return nil, nil, e
	}
	return c, ch, nil
}

type Publisher struct {
	mu       sync.Mutex
	c        *amqp.Connection
	ch       *amqp.Channel
	confirms chan amqp.Confirmation
	returns  chan amqp.Return
}

func (p *Publisher) Close() {
	if p.c != nil {
		_ = p.c.Close()
	}
	p.ch = nil
	p.c = nil
}
func (p *Publisher) Publish(ctx context.Context, key string, b []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch == nil || p.c.IsClosed() {
		p.Close()
		c, ch, e := Connect()
		if e != nil {
			return e
		}
		p.c, p.ch = c, ch
		if e = ch.Confirm(false); e != nil {
			p.Close()
			return e
		}
		p.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
		p.returns = ch.NotifyReturn(make(chan amqp.Return, 1))
	}
	e := p.ch.PublishWithContext(ctx, Exchange, key, key == "telemetry.received.v1", false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: b})
	if e != nil {
		p.Close()
		return e
	}
	select {
	case c, ok := <-p.confirms:
		if !ok || !c.Ack {
			p.Close()
			return fmt.Errorf("broker did not confirm")
		}
		select {
		case <-p.returns:
			return fmt.Errorf("message unroutable")
		default:
			return nil
		}
	case <-ctx.Done():
		p.Close()
		return ctx.Err()
	}
}
func Outbox(ctx context.Context, db *pgxpool.Pool, owner string) {
	p := &Publisher{}
	defer p.Close()
	for ctx.Err() == nil {
		tx, e := db.Begin(ctx)
		if e != nil {
			Pause(ctx, time.Second)
			continue
		}
		var id, key string
		var body json.RawMessage
		e = tx.QueryRow(ctx, `SELECT id::text,event_type,jsonb_build_object('event_id',id,'type',event_type,'occurred_at',created_at,'data',payload) FROM outbox_events WHERE owner=$1 AND published_at IS NULL ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`, owner).Scan(&id, &key, &body)
		if e != nil {
			tx.Rollback(ctx)
			Pause(ctx, 250*time.Millisecond)
			continue
		}
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		e = p.Publish(c, key, body)
		cancel()
		if e == nil {
			_, e = tx.Exec(ctx, "UPDATE outbox_events SET published_at=now() WHERE id=$1", id)
		}
		if e == nil {
			e = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if e != nil {
			slog.Warn("outbox retry", "error", e)
			Pause(ctx, time.Second)
		}
	}
}
func Pause(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
