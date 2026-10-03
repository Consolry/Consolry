package panel

import (
	"strings"
	"testing"
)

// Made by the website's code with the real signing key, for a Host plan with 8 nodes.
const sampleLicence = "CONSOLRY-eyJzdWIiOiJzdWJfMTIzIiwiY3VzIjoiY3VzXzkiLCJwbGFuIjoiaG9zdCIsIm5vZGVzIjo4LCJpYXQiOjE3OTA5ODY0ODgsImV4cCI6MjAwMDYwNDgwMH0.mH0ChZf9iJWs5UdNdIS96po8DVtbcIRcIgFGW1O3zOr5ZAZKNkxHIJXneHEAffFdZb3JRRisrm1AMcaZdInPCA"

func TestLicenceKeys(t *testing.T) {
	licence, err := readLicence(sampleLicence)
	if err != nil || licence.Plan != "host" || licence.Nodes != 8 || licence.Subscription != "sub_123" {
		t.Fatalf("got %+v, %v", licence, err)
	}
	// Changing any part of the key breaks the signature.
	body, signature, _ := strings.Cut(strings.TrimPrefix(sampleLicence, "CONSOLRY-"), ".")
	tampered := "CONSOLRY-" + strings.Replace(body, "aG9zd", "cHJvI", 1) + "." + signature
	for _, key := range []string{tampered, sampleLicence[:len(sampleLicence)-4] + "AAAA", "CONSOLRY-abc.def", "", "nonsense"} {
		if _, err := readLicence(key); err == nil {
			t.Errorf("key %q should be refused", key)
		}
	}
}
