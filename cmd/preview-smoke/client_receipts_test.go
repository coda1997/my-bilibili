package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"my-bilibili/internal/previewbundle"
)

func clientReceiptBytes(t *testing.T, event string, o clientObserved) []byte {
	t.Helper()
	b, err := json.Marshal(previewbundle.ClientReceipt{Event: event, Observation: o})
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func TestClientReceiptStreamHandlesChunksOrderAndSafeRetention(t *testing.T) {
	r := newClientReceipts()
	minimized := completeClientObservation()
	minimized.Minimized = true
	data := append([]byte("private-unstructured-diagnostic\n"), clientReceiptBytes(t, previewbundle.ClientWindowMinimized, minimized)...)
	data = append(data, clientReceiptBytes(t, previewbundle.ClientWindowClosed, closedClientSample())...)
	for _, b := range data {
		if n, err := r.Write([]byte{b}); err != nil || n != 1 {
			t.Fatal(n, err)
		}
	}
	if got, err := r.get(previewbundle.ClientWindowMinimized); err != nil || got != minimized {
		t.Fatal("minimized sample lost", err)
	}
	if got, err := clientClosed(&ownedProcess{clientReceipts: r}); err != nil || got != closedClientSample() {
		t.Fatal("closed sample lost", err)
	}
	encoded, _ := json.Marshal(r.values)
	if strings.Contains(string(encoded), "private") || len(r.pending) != 0 {
		t.Fatal("raw output was retained")
	}
}

func TestClientReceiptStreamRejectsMissingMalformedAndLeakingClose(t *testing.T) {
	valid := clientReceiptBytes(t, previewbundle.ClientWindowClosed, closedClientSample())
	minimized := completeClientObservation()
	minimized.Minimized = true
	leak := closedClientSample()
	leak.Resources.Workers = 1
	for name, data := range map[string][]byte{
		"missing":             nil,
		"duplicate":           append(bytes.Clone(valid), valid...),
		"duplicate-minimized": append(append(clientReceiptBytes(t, previewbundle.ClientWindowMinimized, minimized), clientReceiptBytes(t, previewbundle.ClientWindowMinimized, minimized)...), valid...),
		"reordered":           append(bytes.Clone(valid), clientReceiptBytes(t, previewbundle.ClientWindowMinimized, minimized)...),
		"incomplete":          bytes.TrimSuffix(valid, []byte{'\n'}),
		"trailing-incomplete": append(bytes.Clone(valid), []byte("{")...),
		"malformed":           []byte("{\"Event\":\"private-sentinel\"}\n"),
		"overflow":            append(append(bytes.Clone(valid), bytes.Repeat([]byte{'x'}, 65537)...), '\n'),
		"leak":                clientReceiptBytes(t, previewbundle.ClientWindowClosed, leak),
	} {
		t.Run(name, func(t *testing.T) {
			r := newClientReceipts()
			if n, err := r.Write(data); err != nil || n != len(data) {
				t.Fatal("collector stopped draining")
			}
			if _, err := r.get(previewbundle.ClientWindowClosed); err == nil || strings.Contains(err.Error(), "private") {
				t.Fatal("invalid stream passed or exposed content", err)
			}
		})
	}
	if _, err := (*clientReceipts)(nil).get(previewbundle.ClientWindowClosed); err == nil {
		t.Fatal("missing collector became closed evidence")
	}
}

func TestClientEnvironmentKeepsFixedScenariosAndOwnedHistory(t *testing.T) {
	in := []string{"HOME=/tmp/home", "MY_BILIBILI_E2E_CLIENT_SCENARIO=foreign", "MY_BILIBILI_E2E_HISTORY_DIR=/private", "HTTP_PROXY=private", "DYLD_INSERT_LIBRARIES=private"}
	for _, scene := range []string{"search", "playback", "segments", "history", "account-expiry", "account-signout", "history-corrupt", "history-in-use", "close-held"} {
		env, err := scenarioEnvironment(in, "/owned/socket", "", scene, "/owned/history")
		joined := strings.Join(env, "\n")
		if err != nil || !strings.Contains(joined, "MY_BILIBILI_E2E_CLIENT_SCENARIO="+scene) || !strings.Contains(joined, "MY_BILIBILI_E2E_HISTORY_DIR=/owned/history") || strings.Contains(joined, "private") || strings.Contains(joined, "foreign") {
			t.Fatal("client environment not owned", err)
		}
	}
	for _, args := range [][3]string{{"delayed", "search", "/owned"}, {"", "arbitrary", "/owned"}, {"", "search", "relative"}, {"", "", "/owned"}} {
		if _, err := scenarioEnvironment(in, "/owned/socket", args[0], args[1], args[2]); err == nil {
			t.Fatal("unassigned/mixed client selection accepted")
		}
	}
	env, err := scenarioEnvironment(in, "/owned/socket", "source-detail", "", "")
	if err != nil || strings.Contains(strings.Join(env, "\n"), "MY_BILIBILI_E2E_CLIENT") {
		t.Fatal("fixture mode inherited a client selector", err)
	}
}
