package parser

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
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
			cmd, err := strconv.Unquote(`"` + m[2] + `"`) // エスケープ解除
			if err != nil {
				cmd = m[2] // 失敗したら元のまま
			}
			aliases = append(aliases, Alias{
				Name: m[1],
				Cmd:  cmd,
			})
		}
	}
	return aliases, scanner.Err()
}
