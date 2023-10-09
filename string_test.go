package chaff_test

import (
	"testing"

	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestFormat(t *testing.T){
	t.Parallel()
	test.TestJsonSchema(t, "test_data/string/format_date_time.json", 100)
	test.TestJsonSchema(t, "test_data/string/format_time.json", 100)
	test.TestJsonSchema(t, "test_data/string/format_date.json", 100)
	test.TestJsonSchema(t, "test_data/string/format_email.json", 100)
	test.TestJsonSchema(t, "test_data/string/format_hostname.json", 100)
	test.TestJsonSchema(t, "test_data/string/format_ipv4.json", 100)
	test.TestJsonSchema(t, "test_data/string/format_ipv6.json", 100)
	test.TestJsonSchema(t, "test_data/string/format_uri.json", 100)
	
	// @todo validate beyond draft 7
	// test.TestJsonSchema(t, "test_data/string/format_duration.json", 100)
	// test.TestJsonSchema(t, "test_data/string/format_idn_email.json", 100)
	// test.TestJsonSchema(t, "test_data/string/format_uuid.json", 100)
	// test.TestJsonSchema(t, "test_data/string/format_idn_hostname.json", 100)
	

}