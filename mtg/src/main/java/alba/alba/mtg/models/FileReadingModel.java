package alba.alba.mtg.models;

import java.io.IOException;
import java.io.RandomAccessFile;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import jakarta.annotation.PostConstruct;

@Component
public class FileReadingModel {
    private static final Logger log = LoggerFactory.getLogger(FileReadingModel.class);

    @Value("${cards.file-path}")
    private String dataFile;
    private final ObjectMapper objectMapper = new ObjectMapper();
    private final Map<String, Long> dictId = new HashMap<>();
    private final Map<String, List<Long>> dictName = new HashMap<>();

    @PostConstruct
    private void init() {
        log.info("FileReadingModel init start: {}", dataFile);

        Path path = Path.of(dataFile).toAbsolutePath().normalize();
        if (!Files.exists(path)) {
            throw new IllegalStateException("cards file not found: " + path);
        }

        try {
            readFile(path.toString());
            log.info("FileReadingModel init done");
        } catch (IOException e) {
            throw new RuntimeException("Failed to load cards file: " + path, e);
        }
    }

    private void readFile(String filePath) throws IOException {
        try (RandomAccessFile raf = new RandomAccessFile(filePath, "r")) {
            while (true) {
                long offset = raf.getFilePointer();
                String line = raf.readLine();
                if (line == null) {
                    return;
                }

                String normalized = normalizeLine(line);
                if (!shouldSkipLine(normalized)) {
                    indexLine(normalized, offset);
                }
            }
        } catch (Exception e) {
            log.error("Error while reading cards file", e);
        }
    }
    private String stripTrailingCommas(String s) {
        int end = s.length();
        while (end > 0 && s.charAt(end - 1) == ',') {
            end--;
        }
        return (end == s.length()) ? s : s.substring(0, end);
    }
    public Map<String, Long> getAllDictId() {
        return dictId;
    }

    public Map<String, List<Long>> getAllDictName(){
        return dictName;
    }
    
    private String normalizeLine(String line) {
        return stripTrailingCommas(line.trim());
    }
    private boolean shouldSkipLine(String line) {
        return line.isEmpty() || line.equals("[") || line.equals("]");
    }
    private void indexLine(String line, long offset) throws IOException {
        JsonNode node = objectMapper.readTree(line);
        if (!node.hasNonNull("id")) {
            return;
        }

        String id = node.get("id").asText();
        String name = node.path("name").asText(null);

        if (id != null && !id.isBlank()) {
            dictId.put(id, offset);
        }
        if (name != null && !name.isBlank()) {
            String key = name.toLowerCase();
            dictName.computeIfAbsent(key, k -> new ArrayList<>()).add(offset);
        }
    }

    public List<Long> getOffsetsById(String id) {
        Long offset = dictId.get(id);
        if (offset == null) {
            return List.of();
        }
        return List.of(offset);
    }

    public List<Long> getOffsetsByName(String name) {
        if (name == null) return List.of();
        return dictName.getOrDefault(name.toLowerCase(), List.of());
    }

    public int getTotalIndexedIds() {
        return dictId.size();
    }
}
