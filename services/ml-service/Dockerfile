FROM python:3.12.10-slim
WORKDIR /app
COPY services/ml-service/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY services/ml-service/ .
RUN python train.py && python -m pytest -q && python -m ruff check .
RUN useradd --uid 10001 --create-home app
USER app
ENV RUFF_CACHE_DIR=/tmp/ruff PYTEST_ADDOPTS="-p no:cacheprovider" PYTHONDONTWRITEBYTECODE=1
EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=5s CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/health/ready')"
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
