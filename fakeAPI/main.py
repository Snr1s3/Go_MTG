import json

from fastapi import FastAPI

app = FastAPI()


with open('file.json', 'r') as f:
    data = json.load(f)

@app.get("/health")
def health():
    return {"status": "ok"}


@app.get("/cards/{card_id}")
def get_card(card_id: str):
    return data.get(card_id,{"ERROR": "CARD NOT FOUND"})