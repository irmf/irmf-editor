package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type irmf struct {
	Author    string        `json:"author"`
	Copyright string        `json:"copyright"`
	Date      string        `json:"date"`
	Encoding  *string       `json:"encoding,omitempty"`
	IRMF      string        `json:"irmf"`
	Materials []string      `json:"materials"`
	Max       []float64     `json:"max"`
	Min       []float64     `json:"min"`
	Notes     string        `json:"notes"`
	Options   editorOptions `json:"options"`
	Title     string        `json:"title"`
	Units     string        `json:"units"`
	Version   string        `json:"version"`
}

type editorOptions struct {
	Resolution *int  `json:"resolution,omitempty"`
	Color1     *rgba `json:"color1,omitempty"`
	Color2     *rgba `json:"color2,omitempty"`
	Color3     *rgba `json:"color3,omitempty"`
	Color4     *rgba `json:"color4,omitempty"`
	Color5     *rgba `json:"color5,omitempty"`
	Color6     *rgba `json:"color6,omitempty"`
	Color7     *rgba `json:"color7,omitempty"`
	Color8     *rgba `json:"color8,omitempty"`
	Color9     *rgba `json:"color9,omitempty"`
	Color10    *rgba `json:"color10,omitempty"`
	Color11    *rgba `json:"color11,omitempty"`
	Color12    *rgba `json:"color12,omitempty"`
	Color13    *rgba `json:"color13,omitempty"`
	Color14    *rgba `json:"color14,omitempty"`
	Color15    *rgba `json:"color15,omitempty"`
	Color16    *rgba `json:"color16,omitempty"`
}

type rgba [4]float64

var (
	jsonKeys = []string{
		"author",
		"copyright",
		"date",
		"irmf",
		"materials",
		"max",
		"min",
		"notes",
		"options",
		"title",
		"units",
		"version",
	}
	trailingCommaRE = regexp.MustCompile(`,[\s\n]*}`)
	arrayRE         = regexp.MustCompile(`\[([^\]]+)\]`)
	whitespaceRE    = regexp.MustCompile(`[\s\n]+`)
)

func parseJSON(s string) (*irmf, error) {
	result := &irmf{}

	// Avoid the trailing comma silliness in JavaScript:
	s = trailingCommaRE.ReplaceAllString(s, "}")

	if err := json.Unmarshal([]byte(s), result); err != nil {
		for _, key := range jsonKeys {
			s = strings.Replace(s, key+":", fmt.Sprintf("%q:", key), 1)
		}
		if err := json.Unmarshal([]byte(s), result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (i *irmf) validate(jsonBlobStr, shaderSrc string) (int, error) {
	if i.IRMF != "1.0" {
		return findKeyLine(jsonBlobStr, "irmf"), fmt.Errorf("unsupported IRMF version: %v", i.IRMF)
	}
	if len(i.Materials) < 1 {
		return findKeyLine(jsonBlobStr, "materials"), errors.New("must list at least one material name")
	}
	if len(i.Materials) > 16 {
		return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("IRMF 1.0 only supports up to 16 materials, found %v", len(i.Materials))
	}
	if len(i.Max) != 3 {
		return findKeyLine(jsonBlobStr, "max"), fmt.Errorf("max must have only 3 values, found %v", len(i.Max))
	}
	if len(i.Min) != 3 {
		return findKeyLine(jsonBlobStr, "min"), fmt.Errorf("min must have only 3 values, found %v", len(i.Min))
	}
	if i.Units == "" {
		return findKeyLine(jsonBlobStr, "units"), errors.New("units are required by IRMF 1.0 (even though the irmf-editor ignores the units)")
	}
	if i.Min[0] >= i.Max[0] {
		return findKeyLine(jsonBlobStr, "max"), fmt.Errorf("min.x (%v) must be strictly less than max.x (%v)", i.Min[0], i.Max[0])
	}
	if i.Min[1] >= i.Max[1] {
		return findKeyLine(jsonBlobStr, "max"), fmt.Errorf("min.y (%v) must be strictly less than max.y (%v)", i.Min[1], i.Max[1])
	}
	if i.Min[2] >= i.Max[2] {
		return findKeyLine(jsonBlobStr, "max"), fmt.Errorf("min.z (%v) must be strictly less than max.z (%v)", i.Min[2], i.Max[2])
	}

	if len(i.Materials) <= 4 && strings.Index(shaderSrc, "mainModel4") < 0 {
		return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel4' function", len(i.Materials))
	}

	if len(i.Materials) > 4 && len(i.Materials) <= 9 && strings.Index(shaderSrc, "mainModel9") < 0 {
		return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel9' function", len(i.Materials))
	}

	if len(i.Materials) > 9 && len(i.Materials) <= 16 && strings.Index(shaderSrc, "mainModel16") < 0 {
		return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel16' function", len(i.Materials))
	}

	if i.Encoding != nil && *i.Encoding != "" && *i.Encoding != "gzip" && *i.Encoding != "gzip+base64" {
		return findKeyLine(jsonBlobStr, "encoding"), errors.New("Unsupported encoding. Possible values are 'gzip' or 'gzip+base64'")
	}

	return 0, nil
}

func findKeyLine(s, key string) int {
	if i := strings.Index(s, fmt.Sprintf("%q:", key)); i >= 0 {
		return indexToLineNum(s, i)
	}
	if i := strings.Index(s, fmt.Sprintf("%v:", key)); i >= 0 {
		return indexToLineNum(s, i)
	}
	if i := strings.Index(s, key); i >= 0 {
		return indexToLineNum(s, i)
	}
	logf("Falling back to 2: s=%q, key=%q", s, key)
	return 2 // Fall back to top of json blob.
}

func indexToLineNum(s string, offset int) int {
	s = s[:offset]
	return strings.Count(s, "\n") + 1
}

func (i *irmf) format(shaderSrc string) (string, error) {
	buf, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", fmt.Errorf("unable to format IRMF shader: %v", err)
	}

	jsonBlob := string(buf)

	// Clean up the JSON.
	jsonBlob = strings.Replace(jsonBlob, `"options": null,`, `"options": {},`, 1)
	jsonBlob = arrayRE.ReplaceAllStringFunc(jsonBlob, func(s string) string {
		return whitespaceRE.ReplaceAllString(s, "")
	})

	return fmt.Sprintf("/*%v*/\n%v", jsonBlob, shaderSrc), nil
}
