package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSrclibValidate(t *testing.T) {

	validateEndpoint := "/repos/r@c/.srclib-validate"

	c, mock := newTest()

	calledReposGet := mock.Repos.MockGet(t, "r")
	calledReposGetCommit := mock.Repos.MockGetCommit_ByID_NoCheck(t, "c")

	_ = calledReposGetCommit

	// invalid content-type should fail

	req, err := http.NewRequest("PUT", validateEndpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	} else if resp.StatusCode != http.StatusBadRequest {
		t.Fatal("Expected failure due to invalid content-encoding")
	}

	if *calledReposGet {
		t.Fatal("Repos.Get should not have been called")
	}

	// valid JSON should succeed
	var body bytes.Buffer
	json.NewEncoder(&body).Encode(Validate{Warnings: []BuildWarning{BuildWarning{Directory: "/foo/bar", Warning: "bippity boppity boo"}}})

	req, err = http.NewRequest("PUT", validateEndpoint, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	} else if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected success, got %d", resp.StatusCode)
	}

	if !*calledReposGet {
		t.Fatal("Repos.Get should have been called and was not")
	}

	// empty Body should fail
	body.Reset()
	req, err = http.NewRequest("PUT", validateEndpoint, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	} else if resp.StatusCode != http.StatusBadRequest {
		t.Fatal("Expected failure due to empty body")
	}
}