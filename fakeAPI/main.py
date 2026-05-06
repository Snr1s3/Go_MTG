import json
import os
from fastapi import FastAPI

app = FastAPI()

data_file = '/home/ec2-user/Go_MTG/data/all-cards-20260506092337.json'

# Build index: id -> byte offset in file
index = {}

print("Building index...")
with open(data_file, 'r') as f:
    while True:
        offset = f.tell()
        line = f.readline()
        if not line:
            break
        line = line.strip()
        if line in ('[', ']', ''):
            continue
        line = line.rstrip(',')
        try:
            # Only parse id and name to build index, not the whole card
            # Fast scan using string search before full parse
            if '"id"' in line:
                card = json.loads(line)
                card_id = card.get('id')
                if card_id:
                    index[card_id] = offset
        except Exception:
            continue
print(f"Index built: {len(index)} cards")

@app.get("/health")
def health():
    return {"status": "ok"}

@app.get("/cards/{card_id}")
def get_card(card_id: str):
    offset = index.get(card_id)
    if offset is None:
        return {"ERROR": "CARD NOT FOUND"}
    with open(data_file, 'r') as f:
        f.seek(offset)
        line = f.readline().strip().rstrip(',')
        return json.loads(line)