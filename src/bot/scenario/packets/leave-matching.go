// © 2019-2024 Diarkis Inc. All rights reserved.

// Code generated by "./generator ./config"; DO NOT EDIT.
package packets

// Packet Format
//
//	 No.    | Name             | Type      | Size    |
//	--------|------------------|-----------|---------|
//	 0.     | ticketType       | int       | 1 byte  |
//	--------|------------------|-----------|---------|
type LeaveMatchingReq struct {
	TicketType uint8 `json:"ticketType"`
}

func CreateLeaveMatchingReq(values *LeaveMatchingReq) []byte {
	var bytes []byte

	// Append ticketType
	bytes = append(bytes, values.TicketType)

	return bytes
}
func ParseLeaveMatchingReq(payload []byte) *LeaveMatchingReq {
	var parsed LeaveMatchingReq

	// Parse ticketType
	parsed.TicketType = payload[0]

	return &parsed
}
