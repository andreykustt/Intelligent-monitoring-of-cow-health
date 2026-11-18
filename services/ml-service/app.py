from datetime import datetime, timedelta
from pathlib import Path
import math
import joblib
from fastapi import FastAPI
from pydantic import BaseModel, ConfigDict, field_validator, model_validator

app = FastAPI(title="Cow Health anomaly demo")
artifact = joblib.load(Path(__file__).parent / "artifacts/model.joblib")

class PredictionRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")
    cow_id: str
    window_start: datetime
    window_end: datetime
    features: list[float] | None
    feature_schema_version: int

    @field_validator("features")
    @classmethod
    def finite_features(cls, value):
        if value is not None and (len(value) != 9 or not all(math.isfinite(x) for x in value)):
            raise ValueError("Expected nine finite features or null")
        return value

    @model_validator(mode="after")
    def valid_window(self):
        if self.window_start.tzinfo is None or self.window_end.tzinfo is None:
            raise ValueError("Timezone required")
        if self.window_end - self.window_start != timedelta(hours=1):
            raise ValueError("Expected one-hour window")
        return self

@app.get("/health/ready")
def ready():
    return {"ready": True, "model_version": artifact["version"]}

@app.post("/predict")
def predict(req: PredictionRequest):
    if req.features is None or req.feature_schema_version != artifact["schema"]:
        return {"status": "insufficient_data", "anomaly_score": None, "model_version": artifact["version"]}
    score = float(-artifact["model"].decision_function([req.features])[0])
    return {"status": "anomaly" if score > 0 else "no_anomaly",
            "anomaly_score": score, "model_version": artifact["version"]}
