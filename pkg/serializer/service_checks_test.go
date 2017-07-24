package serializer

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentpayload "github.com/DataDog/agent-payload/gogen"
	"github.com/DataDog/datadog-agent/pkg/metrics"
)

func TestMarshalServiceChecks(t *testing.T) {
	serviceChecks := []*metrics.ServiceCheck{{
		CheckName: "test.check",
		Host:      "test.localhost",
		Ts:        1000,
		Status:    metrics.ServiceCheckOK,
		Message:   "this is fine",
		Tags:      []string{"tag1", "tag2:yes"},
	}}

	payload, contentType, err := MarshalServiceChecks(serviceChecks)
	assert.Nil(t, err)
	assert.NotNil(t, payload)
	assert.Equal(t, contentType, "application/x-protobuf")

	newPayload := &agentpayload.ServiceChecksPayload{}
	err = proto.Unmarshal(payload, newPayload)
	assert.Nil(t, err)

	require.Len(t, newPayload.ServiceChecks, 1)
	assert.Equal(t, newPayload.ServiceChecks[0].Name, "test.check")
	assert.Equal(t, newPayload.ServiceChecks[0].Host, "test.localhost")
	assert.Equal(t, newPayload.ServiceChecks[0].Ts, int64(1000))
	assert.Equal(t, newPayload.ServiceChecks[0].Status, int32(metrics.ServiceCheckOK))
	assert.Equal(t, newPayload.ServiceChecks[0].Message, "this is fine")
	require.Len(t, newPayload.ServiceChecks[0].Tags, 2)
	assert.Equal(t, newPayload.ServiceChecks[0].Tags[0], "tag1")
	assert.Equal(t, newPayload.ServiceChecks[0].Tags[1], "tag2:yes")
}

func TestMarshalJSONServiceChecks(t *testing.T) {
	serviceChecks := []metrics.ServiceCheck{{
		CheckName: "my_service.can_connect",
		Host:      "my-hostname",
		Ts:        int64(12345),
		Status:    metrics.ServiceCheckOK,
		Message:   "my_service is up",
		Tags:      []string{"tag1", "tag2:yes"},
	}}

	payload, contentType, err := MarshalJSONServiceChecks(serviceChecks)
	assert.Nil(t, err)
	assert.Equal(t, contentType, "application/json")
	assert.NotNil(t, payload)
	assert.Equal(t, payload, []byte("[{\"check\":\"my_service.can_connect\",\"host_name\":\"my-hostname\",\"timestamp\":12345,\"status\":0,\"message\":\"my_service is up\",\"tags\":[\"tag1\",\"tag2:yes\"]}]\n"))
}