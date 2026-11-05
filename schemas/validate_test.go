package schemas

import "testing"

func TestValidate(t *testing.T) {
	if _, err := Validate("missing.schema.json", []byte(`{}`)); err == nil {
		t.Fatal("unknown schema must fail")
	}
	issues, err := Validate(Catalog, []byte(`{"apiVersion": "aibox.dev/v1alpha1", "kind": "ModelCatalog"}`))
	if err != nil || len(issues) != 1 || issues[0].Field != "/" {
		t.Fatalf("missing models: %v %v", issues, err)
	}
	issues, err = Validate(Profile, []byte(`{`))
	if err != nil || len(issues) != 1 || issues[0].Field != "/" {
		t.Fatalf("invalid JSON: %v %v", issues, err)
	}
}
