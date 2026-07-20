FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app
RUN addgroup --system app && adduser --system --ingroup app --home /nonexistent --no-create-home app

EXPOSE 8001
USER app

CMD ["uvicorn", "app.main:api", "--host", "0.0.0.0", "--port", "8001"]
