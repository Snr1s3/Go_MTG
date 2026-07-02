import json
from fastapi import FastAPI, HTTPException, Query
from urllib.parse import unquote
from collections import defaultdict

app = FastAPI()

data_file = '../Cartes/cards.jsonl'

index_id = {}
index_name = defaultdict(list)
index_name_lower = defaultdict(list)

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
            if '"id"' in line:
                card = json.loads(line)
                card_id = card.get('id')
                name = card.get('name')
                if card_id:
                    index_id[card_id] = offset
                if name:
                    index_name[name].append(offset)
                    index_name_lower[name.casefold()].append(offset)
                
        except Exception:
            continue

@app.get("/health")
def health():
    return {"status": "ok"}

@app.get("/cards/{card_ref}")
def get_card(card_ref: str, limit: int = Query(20, ge=1, le=200), page_offset: int = Query(0, ge=0)):
    decoded_ref = unquote(card_ref).strip()

    id_offset = index_id.get(decoded_ref)
    if id_offset is not None:
        matched_offsets = [id_offset]
    else:
        matched_offsets = index_name_lower.get(decoded_ref.casefold(), [])

    total = len(matched_offsets)
    if total == 0:
        raise HTTPException(status_code=404, detail="CARD NOT FOUND")

    page_offsets = matched_offsets[page_offset : page_offset + limit]

    cards = []
    with open(data_file, "r") as f:
        for byte_offset in page_offsets:
            f.seek(byte_offset)
            line = f.readline().strip().rstrip(",")
            cards.append(json.loads(line))

    return {
        "total": total,
        "limit": limit,
        "offset": page_offset,
        "count": len(cards),
        "cards": cards
    }