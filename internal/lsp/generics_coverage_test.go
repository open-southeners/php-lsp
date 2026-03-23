package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/open-southeners/tusk-php/internal/protocol"
)

func TestHandleInitializeWiresGenericResolvers(t *testing.T) {
	var out bytes.Buffer
	s := NewServer(strings.NewReader(""), &out, log.New(io.Discard, "", 0))

	params, err := json.Marshal(map[string]interface{}{
		"rootUri":      "file://" + testdataPath(),
		"capabilities": map[string]interface{}{},
		"processId":    nil,
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	id := json.RawMessage("1")
	s.handleInitialize(&jsonRPCMessage{ID: &id, Params: params})

	if s.completion == nil || s.hover == nil {
		t.Fatal("expected completion and hover providers to be initialized")
	}
	if s.hover.GenericExprResolver == nil {
		t.Fatal("expected GenericExprResolver to be wired")
	}

	if rt := s.hover.GenericExprResolver("", "", protocol.Position{}, nil); !rt.IsEmpty() {
		t.Fatalf("expected empty generic expression result, got %#v", rt)
	}

	source := `<?php
namespace App;

class Arr {
    /**
     * @template TKey
     * @template TValue
     * @param array<TKey, TValue> $array
     * @return TValue|null
     */
    public static function first($array) {}
}

$rows = [
    ['id' => 1],
    ['id' => 2],
];

$item = Arr::first($rows);
$item;
`
	uri := "file:///generic-hover.php"
	s.index.IndexFile(uri, source)
	var pos protocol.Position
	for i, line := range strings.Split(source, "\n") {
		if strings.Contains(line, "$item;") {
			pos = protocol.Position{Line: i, Character: strings.Index(line, "$item") + 1}
			break
		}
	}
	hover := s.hover.GetHover(uri, source, pos)
	if hover == nil {
		t.Fatal("expected hover result for generic variable")
	}
	if !strings.Contains(hover.Contents.Value, "?array{id: int} $item") {
		t.Fatalf("expected generic variable hover, got:\n%s", hover.Contents.Value)
	}
}
