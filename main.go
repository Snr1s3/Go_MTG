package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Card struct {
	Name           string
	Quantity       int
	Scryfall_ID    string
	Game_Changer   bool
	Mana_cost      string
	Cmc            int
	Type_line      string
	Oracle_text    string
	Colors         []string
	Color_identity []string
	Keywords       []string
	Power          string
	Toughness      string
}

func callApi(id string) ([]byte, error) {
	resp, err := http.Get("http://localhost:8080/cards/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(body))

	return body, nil
}
func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}
func write_cards_csv(cards *[]Card) error {
	f, err := os.Create("output.csv")
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()
	writer.Write([]string{"Name", "Quantity", "Scryfall_ID", "Game Changer", "Mana_cost", "Type_line", "Oracle_text", "Colors", "Color_identity", "Keywords", "Power", "Toughness"})
	for _, card := range *cards {
		writer.Write([]string{
			card.Name,
			strconv.Itoa(card.Quantity),
			card.Scryfall_ID,
			strconv.FormatBool(card.Game_Changer),
			card.Mana_cost,
			card.Type_line,
			card.Oracle_text,
			strings.Join(card.Colors, "|"),
			strings.Join(card.Color_identity, "|"),
			strings.Join(card.Keywords, "|"),
			card.Power,
			card.Toughness,
		})
	}
	return nil
}
func read_cards_csv(cards *[]Card, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Read() // skip header
	for {
		fields, err := reader.Read()
		if err != nil {
			break
		}
		fmt.Println(fields)
		quantity, _ := strconv.Atoi(fields[6])
		var data map[string]any

		body, err := callApi(fields[8])
		if err != nil {
			fmt.Println("request error:", err)
			return err
		}

		err = json.Unmarshal(body, &data)
		if err != nil {
			fmt.Println("json error:", err)
			return err
		}
		gameChanger, _ := data["game_changer"].(bool)
		manaCost, _ := data["mana_cost"].(string)
		typeLine, _ := data["type_line"].(string)
		oracleText, _ := data["oracle_text"].(string)
		oracleText = strings.ReplaceAll(oracleText, "\n", "\\n")
		power, _ := data["power"].(string)
		toughness, _ := data["toughness"].(string)

		cmcFloat, _ := data["cmc"].(float64)
		cmc := int(cmcFloat)

		card := Card{
			Name:           fields[0],
			Quantity:       quantity,
			Scryfall_ID:    fields[8],
			Game_Changer:   gameChanger,
			Mana_cost:      manaCost,
			Cmc:            cmc,
			Type_line:      typeLine,
			Oracle_text:    oracleText,
			Colors:         toStringSlice(data["colors"]),
			Color_identity: toStringSlice(data["color_identity"]),
			Keywords:       toStringSlice(data["keywords"]),
			Power:          power,
			Toughness:      toughness,
		}
		*cards = append(*cards, card)
	}
	return nil
}
func toStringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if ok {
			result = append(result, s)
		}
	}
	return result
}
func main() {
	fmt.Println("Coleccio de cartes:")
	path := "Cartes3.csv"
	n, err := countLines(path)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("lines:", n)
	cards := []Card{}
	read_cards_csv(&cards, path)
	write_cards_csv(&cards)
}
