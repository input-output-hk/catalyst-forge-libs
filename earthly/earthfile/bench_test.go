package earthfile

import (
	"strconv"
	"strings"
	"testing"
)

// sampleEarthfileSmall is a small Earthfile for parse benchmarks
const sampleEarthfileSmall = `
VERSION 0.8

common:
	FROM alpine:3.18
	RUN echo hello

target1:
	FROM +common
	RUN echo target1
`

// sampleEarthfileMedium simulates multiple targets
func sampleEarthfileMedium(n int) string {
	var b strings.Builder
	b.WriteString("VERSION 0.8\n\n")
	b.WriteString("common:\n\tRUN echo common\n\n")
	for i := 0; i < n; i++ {
		b.WriteString("target" + strconv.Itoa(i) + ":\n")
		b.WriteString("\tFROM +common\n")
		b.WriteString("\tRUN echo t\n")
		b.WriteString("\tSAVE ARTIFACT ./file.txt\n\n")
	}
	return b.String()
}

func BenchmarkParseSmall(b *testing.B) {
	content := sampleEarthfileSmall
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ParseString(content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTargetLookup(b *testing.B) {
	content := sampleEarthfileMedium(100)
	ef, err := ParseString(content)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ef.Target("target50")
	}
}

func BenchmarkTraversal(b *testing.B) {
	content := sampleEarthfileMedium(100)
	ef, err := ParseString(content)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ef.WalkCommands(func(c *Command, depth int) error { return nil })
	}
}
