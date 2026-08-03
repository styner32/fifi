package domesticbond

import (
	"context"
	"net/http"
	"testing"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
)

func TestInquirePrice(t *testing.T) {
	transport := testhelpers.NewMockTransport()
	transport.New("https://example.test").
		Get("/uapi/domestic-bond/v1/quotations/inquire-price?FID_COND_MRKT_DIV_CODE=B&FID_INPUT_ISCD=KR6000291999").
		Reply(http.StatusOK).
		Header("content-type", "application/json").
		BodyString(`{
			"rt_cd": "0",
			"msg_cd": "SUCCESS",
			"msg1": "Success",
			"output": {
				"bond_prpr": "10050.25",
				"bond_name": "국고0300-2909"
			}
		}`)

	client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
	client.Client = &http.Client{Transport: transport}
	client.AuthToken = "dummy-token"

	svc := NewService(client)
	resp, err := svc.InquirePrice(context.Background(), "B", "KR6000291999")
	if err != nil {
		t.Fatalf("InquirePrice error: %v", err)
	}

	if !resp.IsOK() {
		t.Errorf("expected response to be OK, got status %v", resp.IsOK())
	}

	if resp.MessageCode() != "SUCCESS" {
		t.Errorf("expected message code SUCCESS, got %s", resp.MessageCode())
	}

	if err := transport.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestInquirePriceRequiredField(t *testing.T) {
	client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
	svc := NewService(client)

	_, err := svc.InquirePrice(context.Background(), "B", "")
	if err == nil {
		t.Error("expected error for empty inputISCD, got nil")
	}
}
