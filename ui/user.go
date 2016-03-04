package ui

import (
	"encoding/json"
	"fmt"
	"net/http"

	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/ui/payloads"
	"src.sourcegraph.com/sourcegraph/util/handlerutil"
)

func serveUserKeys(w http.ResponseWriter, r *http.Request) error {
	ctx, cl := handlerutil.Client(r)
	e := json.NewEncoder(w)

	currentUser := handlerutil.UserFromRequest(r)
	if currentUser == nil {
		return fmt.Errorf("user not found")
	}

	// Handle adding a key.
	if r.Method == "POST" {
		var data = struct {
			Key, Name string
		}{}

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&data)
		if err != nil {
			return err
		}

		key := sourcegraph.SSHPublicKey{
			Key:  []byte(data.Key),
			Name: data.Name,
		}

		_, err = cl.UserKeys.AddKey(ctx, &key)
		if err != nil {
			return err
		}
	}

	// Handle deleting a key.
	if r.Method == "DELETE" {
		// Decode query parameters.
		ev := struct {
			ID uint64
		}{}
		if err := schemaDecoder.Decode(&ev, r.URL.Query()); err != nil {
			return err
		}

		// Delete the key.
		_, err := cl.UserKeys.DeleteKey(ctx, &sourcegraph.SSHPublicKey{
			ID: ev.ID,
		})
		if err != nil {
			return err
		}
	}

	// Then return the current key list.
	keys, err := cl.UserKeys.ListKeys(ctx, &pbtypes.Void{})
	if err != nil {
		return err
	}

	output := make([]payloads.UserKeysResult, len(keys.SSHKeys))
	for x, key := range keys.SSHKeys {
		output[x] = payloads.UserKeysResult{
			Key:  string(key.Key),
			Name: key.Name,
			ID:   int(key.ID),
		}
	}

	return e.Encode(&struct {
		Results []payloads.UserKeysResult
	}{
		Results: output,
	})
}
