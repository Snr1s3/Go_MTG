package alba.alba.mtg.controllers;

import java.util.Map;

import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;

@Controller
public class HomeController {
    private static final String MESSAGE_KEY = "message";
    private static final String CURRENT_PAGE_KEY = "currentPage";
    private static final String ALL_CARDS_PAGE = "allcards";
    private static final String RANDOM_CARD_JSON_KEY = "randomCardJson";

    private final CardController cardController;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public HomeController(CardController cardController) {
        this.cardController = cardController;
    }

    @GetMapping("/")
    public String home(Model model) {
        model.addAttribute("title", "MTG App");
        model.addAttribute(MESSAGE_KEY, "Random card fetched from /randomcard.");
        model.addAttribute(CURRENT_PAGE_KEY, "home");

        try {
            Map<String, Object> randomCard = cardController.getRandomCard();
            model.addAttribute(RANDOM_CARD_JSON_KEY, toPrettyJson(randomCard));
        } catch (JsonProcessingException | RuntimeException e) {
            model.addAttribute(RANDOM_CARD_JSON_KEY, "Could not load random card: " + e.getMessage());
        }

        return "index";
    }

    @GetMapping("/allcards")
    @SuppressWarnings("java:S3516")
    public String allcards(
            @RequestParam(name = "cardRef", required = false) String cardRef,
            @RequestParam(defaultValue = "20") int limit,
            @RequestParam(name = "page_offset", defaultValue = "0") int pageOffset,
            Model model
    ) {
        model.addAttribute("title", "MTG App");
        model.addAttribute(CURRENT_PAGE_KEY, ALL_CARDS_PAGE);
        model.addAttribute("cardRef", cardRef == null ? "" : cardRef);

        if (cardRef == null || cardRef.trim().isEmpty()) {
            model.addAttribute(MESSAGE_KEY, "Search cards by id or name.");
            model.addAttribute(RANDOM_CARD_JSON_KEY, "Enter a card id or name and click Search.");
            return ALL_CARDS_PAGE;
        }

        try {
            String query = cardRef.trim();
            Map<String, Object> cards = cardController.getCard(query, limit, pageOffset);
            model.addAttribute(MESSAGE_KEY, "Search results for: " + query);
            model.addAttribute(RANDOM_CARD_JSON_KEY, toPrettyJson(cards));
        } catch (JsonProcessingException | RuntimeException e) {
            model.addAttribute(RANDOM_CARD_JSON_KEY, "Could not load random card: " + e.getMessage());
            model.addAttribute(MESSAGE_KEY, "Search failed.");
        }

        return ALL_CARDS_PAGE;
    }
    private String toPrettyJson(Map<String, Object> payload) throws JsonProcessingException {
        return objectMapper.writerWithDefaultPrettyPrinter().writeValueAsString(payload);
    }
}
