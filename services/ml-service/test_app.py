from fastapi.testclient import TestClient
from app import app

client = TestClient(app)
BASE = {"cow_id": "COW-001", "window_start": "2026-11-11T10:00:00Z",
        "window_end": "2026-11-11T11:00:00Z", "feature_schema_version": 1,
        "features": [38.6, .2, 0, 65, .4, 0, .5, .003, 0]}

def test_deterministic():
    a = client.post("/predict", json=BASE)
    assert a.status_code == 200
    assert a.json() == client.post("/predict", json=BASE).json()
    assert a.json()["status"] == "no_anomaly"

def test_missing_and_version():
    for change in ({"features": None}, {"feature_schema_version": 2}):
        r = client.post("/predict", json=BASE | change)
        assert r.json()["status"] == "insufficient_data"
        assert r.json()["anomaly_score"] is None

def test_validation():
    for change in ({"features": [1]}, {"window_end": "2026-11-11T09:00:00Z"}, {"cow_id": None}):
        assert client.post("/predict", json=BASE | change).status_code == 422

def test_anomaly():
    r = client.post("/predict", json=BASE | {"features": [41, 1, .1, 120, 4, 1, .1, .01, .01]})
    assert r.json()["status"] == "anomaly"
