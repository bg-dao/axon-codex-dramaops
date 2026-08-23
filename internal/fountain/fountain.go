package fountain

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

// Parse converts a deliberately small, production-safe Fountain subset into
// DramaOps script manifests. IDs are deterministic and preserved explicitly in
// exports, so import/export does not silently duplicate scenes or dialogue.
func Parse(episodeID, title, source string) (domain.Episode, []domain.Scene, error) {
	if strings.TrimSpace(episodeID) == "" {
		return domain.Episode{}, nil, errors.New("episode id is required")
	}
	lines := scanLines(source)
	var scenes []domain.Scene
	var blocks []domain.ScriptBlock
	var currentScene *domain.Scene
	pendingID := ""
	pendingCharacterID := ""
	orderByScene := map[string]int{}
	duplicateIDs := map[string]int{}

	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if currentScene == nil && strings.HasPrefix(strings.ToLower(line), "title:") {
			i++
			continue
		}
		if id, ok := metadataID(line); ok {
			pendingID = id
			i++
			continue
		}
		if id, ok := metadataCharacterID(line); ok {
			pendingCharacterID = id
			i++
			continue
		}
		if isSceneHeading(line) {
			id := pendingID
			if id == "" {
				id = deterministicID("scene", episodeID, line, duplicateIDs)
			}
			pendingID = ""
			pendingCharacterID = ""
			scene := domain.Scene{SchemaVersion: domain.SchemaVersion, ID: id, EpisodeID: episodeID, Title: strings.TrimPrefix(line, "."), Order: len(scenes), ShotIDs: []string{}}
			scenes = append(scenes, scene)
			currentScene = &scenes[len(scenes)-1]
			i++
			continue
		}
		if currentScene == nil {
			return domain.Episode{}, nil, fmt.Errorf("content before the first scene heading at line %d", i+1)
		}

		if kind, text, ok := directive(line); ok {
			blocks = append(blocks, newBlock(pendingID, episodeID, currentScene.ID, kind, "", text, "", orderByScene, duplicateIDs))
			pendingID, pendingCharacterID = "", ""
			i++
			continue
		}
		if isCharacterCue(line, lines, i) {
			character, voiceOver := parseCharacterCue(line)
			if pendingCharacterID != "" {
				character = pendingCharacterID
			}
			i++
			emotion := ""
			if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "(") && strings.HasSuffix(strings.TrimSpace(lines[i]), ")") {
				emotion = strings.Trim(strings.TrimSpace(lines[i]), "() ")
				i++
			}
			var dialogue []string
			for i < len(lines) {
				value := strings.TrimSpace(lines[i])
				if value == "" || isSceneHeading(value) {
					break
				}
				dialogue = append(dialogue, value)
				i++
			}
			kind := domain.ScriptDialogue
			if voiceOver {
				kind = domain.ScriptVoiceOver
			}
			blocks = append(blocks, newBlock(pendingID, episodeID, currentScene.ID, kind, character, strings.Join(dialogue, " "), emotion, orderByScene, duplicateIDs))
			pendingID, pendingCharacterID = "", ""
			continue
		}

		var action []string
		for i < len(lines) {
			value := strings.TrimSpace(lines[i])
			if value == "" || isSceneHeading(value) || isCharacterCue(value, lines, i) {
				break
			}
			if _, _, ok := directive(value); ok {
				break
			}
			action = append(action, value)
			i++
		}
		if len(action) == 0 {
			i++
			continue
		}
		blocks = append(blocks, newBlock(pendingID, episodeID, currentScene.ID, domain.ScriptAction, "", strings.Join(action, " "), "", orderByScene, duplicateIDs))
		pendingID, pendingCharacterID = "", ""
	}
	if len(scenes) == 0 {
		return domain.Episode{}, nil, errors.New("Fountain script has no scene headings")
	}
	episode := domain.Episode{SchemaVersion: domain.SchemaVersion, ID: episodeID, Title: title, Status: domain.EpisodePlanning, ScriptBlocks: blocks, SceneIDs: make([]string, len(scenes))}
	for i, scene := range scenes {
		episode.SceneIDs[i] = scene.ID
	}
	return episode, scenes, nil
}

func Format(episode domain.Episode, scenes []domain.Scene, characters []domain.Character) string {
	characterNames := map[string]string{}
	for _, character := range characters {
		characterNames[character.ID] = character.Name
	}
	sceneByID := map[string]domain.Scene{}
	for _, scene := range scenes {
		if scene.EpisodeID == episode.ID {
			sceneByID[scene.ID] = scene
		}
	}
	ordered := make([]domain.Scene, 0, len(episode.SceneIDs))
	for _, id := range episode.SceneIDs {
		if scene, ok := sceneByID[id]; ok {
			ordered = append(ordered, scene)
		}
	}
	if len(ordered) == 0 {
		for _, scene := range scenes {
			if scene.EpisodeID == episode.ID {
				ordered = append(ordered, scene)
			}
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })
	}

	var output strings.Builder
	fmt.Fprintf(&output, "Title: %s\n\n", episode.Title)
	for _, scene := range ordered {
		fmt.Fprintf(&output, "/* dramaops:id=%s */\n.%s\n\n", scene.ID, scene.Title)
		blocks := make([]domain.ScriptBlock, 0)
		for _, block := range episode.ScriptBlocks {
			if block.SceneID == scene.ID {
				blocks = append(blocks, block)
			}
		}
		sort.Slice(blocks, func(i, j int) bool { return blocks[i].Order < blocks[j].Order })
		for _, block := range blocks {
			fmt.Fprintf(&output, "/* dramaops:id=%s */\n", block.ID)
			switch block.Kind {
			case domain.ScriptDialogue, domain.ScriptVoiceOver:
				fmt.Fprintf(&output, "/* dramaops:characterId=%s */\n", block.CharacterID)
				name := characterNames[block.CharacterID]
				if name == "" {
					name = block.CharacterID
				}
				if block.Kind == domain.ScriptVoiceOver {
					name += " (V.O.)"
				}
				fmt.Fprintf(&output, "@%s\n", name)
				if block.Emotion != "" {
					fmt.Fprintf(&output, "(%s)\n", block.Emotion)
				}
				fmt.Fprintf(&output, "%s\n\n", block.Text)
			case domain.ScriptSFX:
				fmt.Fprintf(&output, "[[SFX: %s]]\n\n", block.Text)
			case domain.ScriptMusic:
				fmt.Fprintf(&output, "[[MUSIC: %s]]\n\n", block.Text)
			default:
				fmt.Fprintf(&output, "%s\n\n", block.Text)
			}
		}
	}
	return output.String()
}

func scanLines(source string) []string {
	scanner := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(source, "\r\n", "\n")))
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func metadataID(line string) (string, bool) {
	return metadataValue(line, "id")
}

func metadataCharacterID(line string) (string, bool) {
	return metadataValue(line, "characterId")
}

func metadataValue(line, key string) (string, bool) {
	prefix := "/* dramaops:" + key + "="
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, " */") {
		return "", false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(line, prefix), " */")
	return strings.TrimSpace(value), value != ""
}

func isSceneHeading(line string) bool {
	upper := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(line, ".")))
	for _, prefix := range []string{"INT.", "EXT.", "INT/EXT.", "I/E.", "内景", "外景"} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimSpace(line), ".")
}

func directive(line string) (domain.ScriptBlockKind, string, bool) {
	upper := strings.ToUpper(line)
	for label, kind := range map[string]domain.ScriptBlockKind{"[[SFX:": domain.ScriptSFX, "[[MUSIC:": domain.ScriptMusic} {
		if strings.HasPrefix(upper, label) && strings.HasSuffix(line, "]]") {
			return kind, strings.TrimSpace(line[len(label) : len(line)-2]), true
		}
	}
	return "", "", false
}

func isCharacterCue(line string, lines []string, index int) bool {
	if strings.HasPrefix(line, "@") {
		return true
	}
	if index+1 >= len(lines) || strings.TrimSpace(lines[index+1]) == "" {
		return false
	}
	letters, upper := 0, 0
	for _, r := range line {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	return letters > 0 && letters == upper && len([]rune(line)) <= 48
}

func parseCharacterCue(line string) (string, bool) {
	value := strings.TrimSpace(strings.TrimPrefix(line, "@"))
	upper := strings.ToUpper(value)
	voiceOver := strings.Contains(upper, "(V.O.)") || strings.Contains(upper, "(VO)")
	value = strings.TrimSpace(strings.NewReplacer("(V.O.)", "", "(VO)", "", "(v.o.)", "", "(vo)", "").Replace(value))
	return slug(value), voiceOver
}

func newBlock(explicitID, episodeID, sceneID string, kind domain.ScriptBlockKind, characterID, text, emotion string, orders, duplicates map[string]int) domain.ScriptBlock {
	id := explicitID
	if id == "" {
		id = deterministicID("block", episodeID, string(kind)+"|"+sceneID+"|"+characterID+"|"+text, duplicates)
	}
	order := orders[sceneID]
	orders[sceneID]++
	return domain.ScriptBlock{ID: id, SceneID: sceneID, Kind: kind, Order: order, CharacterID: characterID, Text: strings.TrimSpace(text), Emotion: emotion}
}

func deterministicID(kind, episodeID, value string, duplicates map[string]int) string {
	hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
	base := fmt.Sprintf("%s-%s-%s", episodeID, kind, hex.EncodeToString(hash[:6]))
	count := duplicates[base]
	duplicates[base]++
	if count > 0 {
		return fmt.Sprintf("%s-%02d", base, count+1)
	}
	return base
}

func slug(value string) string {
	var result strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		} else if result.Len() > 0 && !strings.HasSuffix(result.String(), "-") {
			result.WriteByte('-')
		}
	}
	return strings.Trim(result.String(), "-")
}
