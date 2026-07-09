package alba.alba.mtg.controllers;

import java.util.List;
import java.util.Map;

import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import alba.alba.mtg.models.FileReadingModel;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;

@RestController
@Validated
public class CardController {
    private final FileReadingModel fr;

    public CardController(FileReadingModel fr) {
        this.fr = fr;
    }

    @GetMapping("/cards/{card_ref}")
    public Map<String, Object> getCard(
            @PathVariable("card_ref") String cardRef,
            @RequestParam(defaultValue = "20") @Min(1) @Max(200) int limit,
            @RequestParam(name = "page_offset", defaultValue = "0") @Min(0) int pageOffset
    ) {
        String decodedRef = java.net.URLDecoder.decode(cardRef, java.nio.charset.StandardCharsets.UTF_8).trim();

        List<Long> matchedOffsets = fr.getOffsetsById(decodedRef);
        if (matchedOffsets.isEmpty()) {
            matchedOffsets = fr.getOffsetsByName(decodedRef);
        }

        int total = matchedOffsets.size();
        if (total == 0) {
            throw new org.springframework.web.server.ResponseStatusException(
                    org.springframework.http.HttpStatus.NOT_FOUND,
                    "CARD NOT FOUND"
            );
        }

        int from = Math.min(pageOffset, total);
        int to = Math.min(pageOffset + limit, total);
        List<Long> pageOffsets = matchedOffsets.subList(from, to);

        ObjectMapper objectMapper = new ObjectMapper();
        List<JsonNode> cards = new java.util.ArrayList<>();

        try (java.io.RandomAccessFile raf = new java.io.RandomAccessFile("mtg/src/main/resources/test.jsonl", "r")) {
            for (Long byteOffset : pageOffsets) {
                raf.seek(byteOffset);
                String line = raf.readLine();
                if (line == null) {
                    continue;
                }

                line = line.trim();
                int end = line.length();
                while (end > 0 && line.charAt(end - 1) == ',') {
                    end--;
                }
                line = (end == line.length()) ? line : line.substring(0, end);

                cards.add(objectMapper.readTree(line));
            }
        } catch (java.io.IOException e) {
            throw new org.springframework.web.server.ResponseStatusException(
                    org.springframework.http.HttpStatus.INTERNAL_SERVER_ERROR,
                    "Failed to read cards",
                    e
            );
        }

        return Map.of(
                "total", total,
                "limit", limit,
                "offset", pageOffset,
                "count", cards.size(),
                "cards", cards
        );
    }
}