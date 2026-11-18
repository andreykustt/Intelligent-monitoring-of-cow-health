# Cow Health Monitor — business requirements (MVP draft)

> Status: working draft for business-analysis alignment, not an approved farm deployment specification. The items below summarize intended business needs; technical implementation remains governed by the existing architecture, data contracts, ML scope and acceptance documents.

## 1. Business context and problem

Dairy farms need timely visibility into animal condition to prioritize veterinary checks and reduce avoidable operational losses associated with late recognition of health problems. Relevant losses may include treatment, discarded or reduced milk output, additional labor and premature culling. These are **potential loss categories**, not measured losses for a particular farm.

**Reference business case:** a herd of **3,500 cows** at an EcoNiva farm, per the project stakeholder's working assumption. This is an economic-planning scenario, **not** proof of current farm data, sensor coverage, or tested MVP capacity. Confirm the number of lactating cows separately when modeling mastitis.

**Business proposition:** make unusual patterns in incoming measurements visible to a farm operator earlier, so staff can decide which animals need examination. The system supports human decision-making; it does not diagnose disease or recommend treatment.

## 2. Target users and stakeholders

| Stakeholder | Business need | MVP system role |
|---|---|---|
| Farm manager / buyer | Understand herd-level trends, possible avoided losses and value of deployment | May review outputs; **no separate role in MVP** |
| Veterinarian / livestock-health specialist | Identify animals needing a closer check and review their history | `operator` account where access is granted |
| Farm employee recording examinations and milking data | Enter observations consistently, view alerts and record follow-up | `operator` |
| Farm system administrator | Manage users, animals and demonstration settings | `admin` |

Only `admin` and `operator` are application roles in the MVP. A business stakeholder is not automatically a new software role.

## 3. Process outline (to validate with a farm representative)

**AS-IS hypothesis:** staff collect observations and milking/laboratory results in separate steps; suspicious changes may be noticed during periodic checks; a veterinarian reviews the animal and records a conclusion. The actual EcoNiva workflow, frequencies, IT systems and delay to examination **have not been verified**.

**TO-BE (MVP demonstration):** simulated/replayed sensor events and manually entered observations appear in one cow profile -> an anomaly is flagged when sufficient supported evidence exists -> an operator reviews measurements and timestamps -> an operator acknowledges the alert and arranges follow-up outside the system -> veterinary findings, if available, are recorded manually. The alert itself is **not** a confirmed disease case.

**Intended production benefit to test later:** shorter time between an abnormal pattern and a human assessment; fewer preventable losses if appropriate interventions follow. A real-farm pilot and clinical reference data are required to verify this effect.

## 4. Objectives and success measures

**MVP SMART objective (technical/business-process validation):** by completion of milestone **M7**, demonstrate on the baseline simulation of **200 cows** that an authorized operator can view current/history data, enter manual records, review an anomaly warning and acknowledge/resolve it; complete the existing acceptance checks and report observed load-test measurements. The **1,000-cow** scenario is extended testing, not a production guarantee.

**Long-term business hypothesis (not an MVP acceptance promise):** within **12 months after a real-farm deployment**, aim to reduce *veterinarian-confirmed mastitis incidence* by **25% relative to an agreed baseline**, measured as cases per 100 lactating cows per year, while tracking alert workload and examination timeliness. The target, baseline, timeframe and causal attribution require agreement with the farm and evidence from a pilot.

**Indicative KPIs:**
- Operational (MVP-observable): time from event receipt to alert creation; time from alert creation to acknowledgment; share of stale/missing readings; number of alerts and repeated alerts per cow; percentage of alerts with a recorded operator action.
- Clinical/business (not measurable with synthetic MVP data): confirmed mastitis cases per 100 lactating cows/year; milk lost or discarded; treatment and extra labor cost; premature culling; cost per cow monitored; annual net savings.

Do **not** treat an ML `anomaly` label or model score as a disease diagnosis or clinical risk probability.

## 5. Functional requirements

| ID | Requirement | Priority / scope |
|---|---|---|
| BA-FR-01 | An authorized user can view current readings, data freshness and recorded source for cows in their permitted farm scope. | Must / MVP |
| BA-FR-02 | An operator can open an individual cow profile with historical readings and available manual observations. | Must / MVP |
| BA-FR-03 | An operator can add and correct milking, visual-inspection, laboratory, body-condition/hoof and veterinary-examination records, with timestamps and audit trail. | Must / MVP |
| BA-FR-04 | The system presents `anomaly`, `no_anomaly` or `insufficient_data` on supported features, creates/deduplicates actionable alerts and allows operators to acknowledge/resolve them. | Must / MVP |
| BA-FR-05 | The herd monitor supports sorting, filtering and pagination; available analytics summarize measurements/alerts without mislabeling anomalies as clinical disease cases. | Must / MVP |
| BA-FR-06 | Only admins may manage users, animal records and simulations; operators have only the permitted monitoring/manual-entry/alert actions. | Must / MVP |
| BA-FR-07 | Admins can run and control synthetic/replay demonstrations. Actual hardware ingestion and verified monitoring of 3,500 cows are **out of MVP scope**. | Must / MVP boundary |

These requirements restate or organize the already planned behavior; implementation details remain in `architecture.md`, `data-contracts.md`, `ml.md` and the API specification.

## 6. Non-functional requirements and constraints

| ID | Requirement / constraint | How to check |
|---|---|---|
| BA-NFR-01 | Distinguish synthetic, replay and manually entered data; show event/observation time, source and freshness, without presenting simulated data as medical evidence. | UI/DB acceptance checks |
| BA-NFR-02 | Enforce `admin`/`operator` permissions and farm scope on server-side APIs and WebSocket; preserve manual correction history. | Negative authorization and audit tests |
| BA-NFR-03 | Telemetry storage remains operational when ML is unavailable; failed/out-of-order/repeated events do not silently create false success or duplicates. | Failure and recovery scenarios |
| BA-NFR-04 | Report tested throughput, p95 API latency, WS lag and environment for 200 cows, plus 1,000-cow extended load. No numerical production SLA or 3,500-cow capacity claim before measurements. | Documented load test |
| BA-NFR-05 | Alerts must clearly communicate uncertainty and unsupported/missing data; users retain responsibility for clinical decisions. | Label and workflow review |

**Assumptions/limits:** no real sensors connected in MVP; only two access roles; the current ML is anomaly detection, not mastitis diagnosis; commercial value and pricing are unvalidated; historical real-farm baselines have not been provided. Do not make a commitment to the specific bolus type, monitoring cost or payback period without supplier and farm input.

## 7. Preliminary economic model (variables pending validation)

Use the **3,500-cow reference herd** only for business sizing. Required input values are **not contained in the current project documentation**.

| Input | What must be established |
|---|---|
| Cow cost | Purchase/replacement price per cow (RUB); account for salvage value if modeling culling losses |
| Population at risk | Number of lactating cows and observation period |
| Baseline mastitis incidence | Veterinarian-confirmed cases per year; define recurrence and counting rules |
| Loss per mastitis case | Treatment + discarded/reduced milk (using an appropriate economic margin) + extra labor + net culling losses, without double counting |
| Deployment cost | Sensor/bolus quantity and price, installation, integration, software, training, maintenance, replacement and support |

**Scenario formulas:**
- Baseline annual cases = annual cases in farm records (or lactating-cow count × validated incidence per cow-year).
- *Hypothetically prevented* cases at the target = baseline annual cases × **0.25**; this is a scenario assumption, not an achieved result.
- Potential annual gross savings = prevented cases × non-overlapping loss per case + independently substantiated additional benefits.
- Net annual savings = gross savings − annual operating costs.
- Simple payback (years) = initial implementation cost ÷ net annual savings, **only if net annual savings > 0**.

Prepare conservative/base/optimistic scenarios once sources and cost assumptions are available. No monetary estimate or ROI claim is made in this draft.

## 8. Boluses and other sensors — decision to research, not an MVP commitment

A rumen bolus is one possible data source for some physiological measurements. Before selecting it, compare boluses, collars/ear tags and existing farm systems against: available **actual** metrics and sampling frequency; animal welfare/installation requirements; purchase and replacement cost; data access/API; battery/service life; farm connectivity; and ability to support the specific operational use case. Do not assume one device measures all metrics listed in `data-contracts.md`.

**MVP decision:** sensor telemetry remains simulated or replayed. Hardware selection and integration are a possible later pilot decision, requiring technical and veterinary input.

## 9. Open business-analysis questions

1. Is the 3,500-cow figure total cattle, adult cows or lactating cows? Which farm/site and period does it describe?
2. What are the farm's documented mastitis incidence, loss categories, vet workflow and response times?
3. What are the cow-replacement cost and economic value of milk lost/discarded, with sources and dates?
4. What sensor data and systems already exist; who pays for devices, installation and ongoing support?
5. What real-farm pilot, comparison period/group and clinical source of truth would be acceptable to test the 25% hypothesis?

These are validation questions, **not blockers to the simulated MVP**.
