//go:build !linux

package device

import (
	"github.com/KarpelesLab/wireguard-loop-go/conn"
	"github.com/KarpelesLab/wireguard-loop-go/rwcancel"
)

func (device *Device) startRouteListener(_ conn.Bind) (*rwcancel.RWCancel, error) {
	return nil, nil
}
