package parser

import (
	"encoding/csv"
	"io"
)

func ReadCSVFromReader(r io.Reader) ([][]string, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, nil
	}

	return records[1:], nil
}
