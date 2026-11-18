"""Offline, reproducible synthetic training; no medical labels or clinical claims."""
import json
from pathlib import Path
import joblib
import numpy as np
from sklearn.ensemble import IsolationForest
from sklearn.pipeline import make_pipeline
from sklearn.preprocessing import StandardScaler

VERSION = "synthetic-iforest-v1"

def train(output: Path = Path("artifacts/model.joblib")):
    rng = np.random.default_rng(42)
    # Independent windows: first 1600 training windows, last 400 held out.
    mean = np.array([38.6, 0.2, 0, 65, 0.4, 0, 0.5, 0.003, 0])
    scale = np.array([0.2, 0.04, 0.002, 2, 0.08, 0.01, 0.03, 0.0007, 0.0001])
    x = rng.normal(mean, scale, size=(2000, 9))
    x[:, [1, 4, 7]] = np.abs(x[:, [1, 4, 7]])
    model = make_pipeline(StandardScaler(), IsolationForest(
        n_estimators=150, contamination=0.03, random_state=42, n_jobs=1))
    model.fit(x[:1600])
    output.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump({"model": model, "version": VERSION, "schema": 1}, output)
    report = {"model_version": VERSION, "source": "generated synthetic independent windows",
              "seed": 42, "train_windows": 1600, "held_out_windows": 400,
              "held_out_flag_fraction": float((model.predict(x[1600:]) == -1).mean()),
              "limitations": "No clinical validation. No disease labels. Feature-level demo only."}
    output.with_suffix(".json").write_text(json.dumps(report, indent=2))
    return report

if __name__ == "__main__":
    print(json.dumps(train(), indent=2))
