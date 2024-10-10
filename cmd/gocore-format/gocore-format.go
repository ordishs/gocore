package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"
)

type Setting struct {
	Key      string
	Group    string
	SortBy   string
	Comments string
	Variants []Variant
	Compact  bool
}

type Variant struct {
	Commented bool
	Key       string
	Value     string
	Comment   string // The comment after the key=value pair
}

func main() {
	var (
		write    bool
		help     bool
		filename string
		in       = os.Stdin
		err      error
	)

	flag.BoolVar(&write, "w", false, "Write to file")
	flag.BoolVar(&help, "h", false, "Help")
	flag.Parse()

	if help {
		flag.PrintDefaults()
		return
	}

	args := flag.Args()

	if len(args) > 0 {
		filename = args[0]

		in, err = os.Open(filename)
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}
		defer in.Close()
	}

	settings, err := readSettings(in)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	sortSettings(settings)

	if filename != "" && write {
		in.Close()

		out, err := os.Create(filename + ".tmp")
		if err != nil {
			fmt.Println("Error creating output file:", err)
			return
		}
		defer out.Close()

		if err := writeSettings(out, settings); err != nil {
			fmt.Println("Error writing file:", err)
			return
		}

		if err := os.Rename(filename+".tmp", filename); err != nil {
			fmt.Println("Error renaming file:", err)
			return
		}
	} else {
		if err := writeSettings(os.Stdout, settings); err != nil {
			fmt.Println("Error writing file:", err)
			return
		}
	}
}

func readSettings(r io.Reader) ([]*Setting, error) {
	var pendingSectionComment string
	var currentGroup string
	var isCompactGroup bool

	settings := make(map[string]*Setting)

	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "# @group:") {
			currentGroup = strings.TrimSpace(strings.TrimPrefix(line, "# @group:"))
			isCompactGroup = strings.HasSuffix(currentGroup, " compact")
			if isCompactGroup {
				currentGroup = strings.TrimSuffix(currentGroup, " compact")
			}
			continue
		}

		if line == "# @endgroup" {
			currentGroup = ""
			isCompactGroup = false
			continue
		}

		item := processLine(line)

		if item == nil {
			// This is an arbitrary comment line
			line = strings.TrimSpace(line[1:])

			if pendingSectionComment == "" {
				pendingSectionComment = line
			} else {
				pendingSectionComment += "\n" + line
			}
		} else {
			rootKey := strings.Split(item.Key, ".")[0]

			setting, found := settings[rootKey]
			if !found {
				setting = &Setting{
					Key:      rootKey,
					Comments: pendingSectionComment,
					Compact:  isCompactGroup,
				}

				if currentGroup != "" {
					setting.Group = currentGroup
					setting.SortBy = currentGroup
				} else {
					setting.SortBy = rootKey
				}

				pendingSectionComment = ""
			}

			setting.Variants = append(setting.Variants, *item)

			settings[rootKey] = setting
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	settingsSlice := make([]*Setting, 0, len(settings))
	for _, setting := range settings {
		settingsSlice = append(settingsSlice, setting)
	}

	return settingsSlice, nil
}

func writeSettings(w io.Writer, settings []*Setting) error {
	writer := bufio.NewWriter(w)
	defer writer.Flush()

	currentGroup := ""
	isCompactGroup := false

	for i, setting := range settings {
		if setting.Group != currentGroup {
			if currentGroup != "" {
				// Remove the extra newline before # @endgroup for non-compact groups
				if !isCompactGroup {
					// Move the cursor back one character to overwrite the extra newline
					writer.WriteString("\033[1A")
				}
				_, err := writer.WriteString("# @endgroup\n\n")
				if err != nil {
					return err
				}
			}
			if setting.Group != "" {
				groupLine := "# @group: " + setting.Group
				if setting.Compact {
					groupLine += " compact"
				}
				_, err := writer.WriteString(groupLine + "\n")
				if err != nil {
					return err
				}
			}
			currentGroup = setting.Group
			isCompactGroup = setting.Compact
		}

		if setting.Comments != "" {
			_, err := writer.WriteString("# " + setting.Comments + "\n")
			if err != nil {
				return err
			}
		}

		maxKeyLength := 0

		for _, variant := range setting.Variants {

			l := len(variant.Key)
			if variant.Commented {
				l += 2
			}

			if l > maxKeyLength {
				maxKeyLength = l
			}
		}

		for _, variant := range setting.Variants {
			prefix := ""

			length := maxKeyLength

			if variant.Commented {
				prefix = "# "
				length -= 2
			}

			value := cleanMultiValues(variant.Value)

			line := fmt.Sprintf("%s%-*s = %s", prefix, length, variant.Key, value)

			if variant.Comment != "" {
				line += " # " + variant.Comment
			}

			_, err := writer.WriteString(line + "\n")
			if err != nil {
				return err
			}
		}

		if !isCompactGroup {
			_, err := writer.WriteString("\n")
			if err != nil {
				return err
			}
		}

		// Check if this is the last setting or if the next setting is in a different group
		if i == len(settings)-1 || settings[i+1].Group != currentGroup {
			if currentGroup != "" {
				// Remove the extra newline before # @endgroup for non-compact groups
				if !isCompactGroup {
					// Move the cursor back one character to overwrite the extra newline
					writer.WriteString("\033[1A")
				}
				_, err := writer.WriteString("# @endgroup\n\n")
				if err != nil {
					return err
				}
			}
			currentGroup = ""
			isCompactGroup = false
		}
	}

	return nil
}

func processLine(line string) *Variant {

	setting := &Variant{}

	if strings.HasPrefix(line, "#") {
		setting.Commented = true
		line = line[1:]
	}

	parts := strings.SplitN(line, "=", 2)

	if len(parts) == 1 {
		return nil
	}

	setting.Key = cleanKey(parts[0])

	line = strings.TrimSpace(parts[1])

	valueParts := strings.SplitN(line, "#", 2)
	setting.Value = strings.TrimSpace(valueParts[0])

	if len(valueParts) > 1 {
		setting.Comment = strings.TrimSpace(valueParts[1])
	}

	return setting
}

func cleanKey(key string) string {
	parts := strings.Split(strings.TrimSpace(key), ".")

	for i := 0; i < len(parts); i++ {
		parts[i] = strings.TrimSpace(parts[i])
	}

	return strings.Join(parts, ".")
}

func cleanMultiValues(value string) string {
	parts := strings.Split(value, "|")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	return strings.Join(parts, " | ")
}

func sortSettings(settings []*Setting) {
	sort.Slice(settings, func(i, j int) bool {
		// First, sort by group
		if settings[i].SortBy != settings[j].SortBy {
			// Handle empty group names
			if settings[i].SortBy == "" {
				return false
			}
			if settings[j].SortBy == "" {
				return true
			}
			r1, r2 := rune(settings[i].SortBy[0]), rune(settings[j].SortBy[0])
			if unicode.IsUpper(r1) != unicode.IsUpper(r2) {
				return unicode.IsUpper(r1)
			}
			return settings[i].SortBy < settings[j].SortBy
		}

		// If groups are the same, sort by key
		keyI := strings.TrimPrefix(settings[i].Key, settings[i].Group+".")
		keyJ := strings.TrimPrefix(settings[j].Key, settings[j].Group+".")

		// Handle empty keys
		if keyI == "" {
			return false
		}
		if keyJ == "" {
			return true
		}

		r1, r2 := rune(keyI[0]), rune(keyJ[0])
		if unicode.IsUpper(r1) != unicode.IsUpper(r2) {
			return unicode.IsUpper(r1)
		}

		return keyI < keyJ
	})
}
