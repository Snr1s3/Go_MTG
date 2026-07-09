package alba.alba.mtg.controllers;

import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;

@Controller
public class HomeController {
    
    @GetMapping("/")
    public String home(Model model) {
        model.addAttribute("title", "MTG App");
        model.addAttribute("message", "Thymeleaf is configured and running.");
        return "index";
        
    }
}
