package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// TableFormatter handles markdown table conversion
type TableFormatter struct {
	lines []string
}

// convertTables converts markdown tables to Slack-friendly format
func convertTables(text string) string {
	lines := strings.Split(text, "\n")
	result := []string{}
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Check if this line looks like a table header
		if strings.Contains(line, "|") && strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			// Found potential table start
			tableLines := []string{}
			j := i

			// Collect all table lines
			for j < len(lines) {
				currentLine := strings.TrimSpace(lines[j])
				if strings.Contains(currentLine, "|") && (strings.HasPrefix(currentLine, "|") || strings.Contains(currentLine, "|")) {
					tableLines = append(tableLines, currentLine)
					j++
				} else if currentLine == "" {
					// Empty line might be part of table formatting
					if j+1 < len(lines) && strings.Contains(lines[j+1], "|") {
						tableLines = append(tableLines, currentLine)
						j++
					} else {
						break
					}
				} else {
					break
				}
			}

			if len(tableLines) >= 2 { // At least header + separator
				// Convert table to formatted text
				formattedTable := formatTableForSlack(tableLines)
				result = append(result, formattedTable)
				i = j
				continue
			}
		}

		result = append(result, line)
		i++
	}

	return strings.Join(result, "\n")
}

// formatTableForSlack formats a markdown table for Slack display
func formatTableForSlack(tableLines []string) string {
	// Remove empty lines and clean up
	cleanLines := []string{}
	for _, line := range tableLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	if len(cleanLines) < 2 {
		return strings.Join(tableLines, "\n") // Return as-is if not a proper table
	}

	// Parse table
	rows := [][]string{}
	separatorRegex := regexp.MustCompile(`^\|[\s\-\|:]+\|$`)

	for _, line := range cleanLines {
		// Skip separator lines (|---|---|)
		if separatorRegex.MatchString(line) {
			continue
		}

		// Split by | and clean up
		parts := strings.Split(line, "|")
		if len(parts) >= 3 { // Should have at least |cell1|cell2|
			cells := []string{}
			for i := 1; i < len(parts)-1; i++ { // Remove first/last empty
				cells = append(cells, strings.TrimSpace(parts[i]))
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
		}
	}

	if len(rows) == 0 {
		return strings.Join(tableLines, "\n")
	}

	// Calculate column widths
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	colWidths := make([]int, maxCols)
	for col := 0; col < maxCols; col++ {
		maxWidth := 0
		for _, row := range rows {
			if col < len(row) && len(row[col]) > maxWidth {
				maxWidth = len(row[col])
			}
		}
		colWidths[col] = maxWidth
	}

	// Format as code block for better alignment
	result := []string{"```"}

	for i, row := range rows {
		formattedRow := []string{}
		for j, cell := range row {
			if j < len(colWidths) {
				formattedRow = append(formattedRow, fmt.Sprintf("%-*s", colWidths[j], cell))
			} else {
				formattedRow = append(formattedRow, cell)
			}
		}

		result = append(result, strings.Join(formattedRow, " | "))

		// Add separator after header
		if i == 0 {
			separator := []string{}
			for _, width := range colWidths {
				separator = append(separator, strings.Repeat("-", width))
			}
			result = append(result, strings.Join(separator, "-|-"))
		}
	}

	result = append(result, "```")
	return strings.Join(result, "\n")
}

// markdownToSlack converts markdown text to Slack formatting
func markdownToSlack(text string) string {
	// Headers - convert to bold
	headerRegex1 := regexp.MustCompile(`^### (.*)$`)
	headerRegex2 := regexp.MustCompile(`^## (.*)$`)
	headerRegex3 := regexp.MustCompile(`^# (.*)$`)
	
	text = headerRegex1.ReplaceAllString(text, "*$1*")
	text = headerRegex2.ReplaceAllString(text, "*$1*")
	text = headerRegex3.ReplaceAllString(text, "*$1*")

	// Bold: **text** -> *text*
	boldRegex := regexp.MustCompile(`\*\*(.*?)\*\*`)
	text = boldRegex.ReplaceAllString(text, "*$1*")

	// Italic: *text* -> _text_ (but be careful not to affect our new bold)
	// First pass: temporarily replace bold asterisks
	boldTempRegex := regexp.MustCompile(`(\*[^*]+\*)`)
	boldMatches := boldTempRegex.FindAllString(text, -1)
	for i, match := range boldMatches {
		placeholder := fmt.Sprintf("BOLD_TEMP%d_TEMP%sTEMP_BOLD", i, match[1:len(match)-1])
		text = strings.Replace(text, match, placeholder, 1)
	}

	// Now convert remaining single asterisks to underscores for italic
	italicRegex := regexp.MustCompile(`\*([^*]+)\*`)
	text = italicRegex.ReplaceAllString(text, "_$1_")

	// Restore bold
	boldRestoreRegex := regexp.MustCompile(`BOLD_TEMP\d+_TEMP(.+?)TEMP_BOLD`)
	text = boldRestoreRegex.ReplaceAllString(text, "*$1*")

	// Code blocks with language - convert to Slack snippets
	codeBlockRegex := regexp.MustCompile("(?s)```\\w*\\n(.*?)```")
	text = codeBlockRegex.ReplaceAllString(text, "```$1```")

	// Inline code stays the same: `code`

	// Unordered lists: convert - to •
	unorderedListRegex := regexp.MustCompile(`(?m)^- `)
	text = unorderedListRegex.ReplaceAllString(text, "• ")
	
	nestedListRegex := regexp.MustCompile(`(?m)^  - `)
	text = nestedListRegex.ReplaceAllString(text, "  ◦ ")

	// Ordered lists: keep numbers but clean up (no changes needed)

	// Links: [text](url) -> text (url)
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	text = linkRegex.ReplaceAllString(text, "$1 ($2)")

	// Blockquotes: > text -> indented text
	blockquoteRegex := regexp.MustCompile(`(?m)^> `)
	text = blockquoteRegex.ReplaceAllString(text, "    ")

	// Tables - convert to formatted text blocks
	text = convertTables(text)

	return text
}

func main() {
	var outputFile string
	flag.StringVar(&outputFile, "o", "", "Output file (default: stdout)")
	flag.StringVar(&outputFile, "output", "", "Output file (default: stdout)")
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [file]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Convert Markdown to Slack formatting\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s file.md\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  echo \"**bold text**\" | %s\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s < input.md > output.txt\n", os.Args[0])
	}
	
	flag.Parse()

	var reader io.Reader
	var inputFile string

	// Determine input source
	if flag.NArg() > 0 {
		inputFile = flag.Arg(0)
		file, err := os.Open(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: File '%s' not found: %v\n", inputFile, err)
			os.Exit(1)
		}
		defer file.Close()
		reader = file
	} else {
		// Check if stdin has data
		stat, err := os.Stdin.Stat()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking stdin: %v\n", err)
			os.Exit(1)
		}
		
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Fprintf(os.Stderr, "Error: No input provided. Use a file argument or pipe input.\n")
			fmt.Fprintf(os.Stderr, "Try: %s --help\n", os.Args[0])
			os.Exit(1)
		}
		reader = os.Stdin
	}

	// Read input
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	markdownText := strings.Join(lines, "\n")

	// Convert
	slackText := markdownToSlack(markdownText)

	// Output
	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		_, err = file.WriteString(slackText)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Converted text written to %s\n", outputFile)
	} else {
		fmt.Print(slackText)
	}
}