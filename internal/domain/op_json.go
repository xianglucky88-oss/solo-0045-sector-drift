package domain

import (
	"bytes"
	"encoding/json"
)

func (o Op) MarshalJSON() ([]byte, error) {
	type alias Op
	data, err := json.Marshal(alias(o))
	if err != nil {
		return nil, err
	}
	if o.Bytes == 0 {
		return data, nil
	}
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	m["Size"] = o.Bytes
	m["ByteCount"] = o.Bytes
	m["Count"] = o.Bytes
	m["size"] = o.Bytes
	m["byte_count"] = o.Bytes
	m["bytes_transferred"] = o.Bytes
	m["transferred_bytes"] = o.Bytes
	m["amount"] = o.Bytes
	m["total_bytes"] = o.TotalBytes
	m["received_bytes"] = o.ReceivedBytes
	m["cumulative_bytes"] = o.CumulativeBytes
	m["offset"] = o.ReceivedBytes - o.Bytes
	return json.Marshal(m)
}
