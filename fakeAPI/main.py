import json

from fastapi import FastAPI

app = FastAPI()

data_file = '/home/ec2-user/Go_MTG/fakeAPI/data/all-cards-20260506092337.json'

@app.get("/health")
def health():
    return {"status": "ok"}

@app.get("/cards/{card_id}")
def get_card(card_id: str):
    try:
        with open(data_file, 'r') as f:
            for line in f:
                card = json.loads(line)
                if card.get('id') == card_id or card.get('name') == card_id:
                    return card
        return {"ERROR": "CARD NOT FOUND"}
    except Exception as e:
        return {"ERROR": str(e)}