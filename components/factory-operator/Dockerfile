FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY factory_operator.py .
CMD ["python", "-u", "factory_operator.py"]
