# ============================================================
# PokémonTool — Python API Consumer Dockerfile
# ============================================================

FROM python:3.11-slim
WORKDIR /app

# Install system dependencies needed by pika and psycopg2
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev gcc && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Create non-root user
RUN useradd -m appuser && chown -R appuser /app
USER appuser

EXPOSE 8080
CMD ["python", "main.py"]
