package parser

import (
	"bufio"
	"os"
	"regexp"
)

var aliasPattern = regexp.MustCompile(`^alias\s+(\w+)=[\"\'](.+)[\"\']`)

type Alias struct {
	Name string
	Cmd  string
}

func ExtractZshAliases(path string) ([]Alias, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var aliases []Alias
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		m := aliasPattern.FindStringSubmatch(line)
		if len(m) == 3 {
			aliases = append(aliases, Alias{
				Name: m[1],
				Cmd:  m[2],
			})
		}
	}
	return aliases, scanner.Err()
}
