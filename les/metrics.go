// Copyright 2016 The go-PaloAltoAi Authors
// This file is part of the go-PaloAltoAi library.
//
// The go-PaloAltoAi library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-PaloAltoAi library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-PaloAltoAi library. If not, see <http://www.gnu.org/licenses/>.

package les

import (
	"github.com/PaloAltoAi/go-PaloAltoAi/metrics"
	"github.com/PaloAltoAi/go-PaloAltoAi/p2p"
)

var (
	/*	propTxnInPacketsMeter     = metrics.NewMeter("paa/prop/txns/in/packets")
		propTxnInTrafficMeter     = metrics.NewMeter("paa/prop/txns/in/traffic")
		propTxnOutPacketsMeter    = metrics.NewMeter("paa/prop/txns/out/packets")
		propTxnOutTrafficMeter    = metrics.NewMeter("paa/prop/txns/out/traffic")
		propHashInPacketsMeter    = metrics.NewMeter("paa/prop/hashes/in/packets")
		propHashInTrafficMeter    = metrics.NewMeter("paa/prop/hashes/in/traffic")
		propHashOutPacketsMeter   = metrics.NewMeter("paa/prop/hashes/out/packets")
		propHashOutTrafficMeter   = metrics.NewMeter("paa/prop/hashes/out/traffic")
		propBlockInPacketsMeter   = metrics.NewMeter("paa/prop/blocks/in/packets")
		propBlockInTrafficMeter   = metrics.NewMeter("paa/prop/blocks/in/traffic")
		propBlockOutPacketsMeter  = metrics.NewMeter("paa/prop/blocks/out/packets")
		propBlockOutTrafficMeter  = metrics.NewMeter("paa/prop/blocks/out/traffic")
		reqHashInPacketsMeter     = metrics.NewMeter("paa/req/hashes/in/packets")
		reqHashInTrafficMeter     = metrics.NewMeter("paa/req/hashes/in/traffic")
		reqHashOutPacketsMeter    = metrics.NewMeter("paa/req/hashes/out/packets")
		reqHashOutTrafficMeter    = metrics.NewMeter("paa/req/hashes/out/traffic")
		reqBlockInPacketsMeter    = metrics.NewMeter("paa/req/blocks/in/packets")
		reqBlockInTrafficMeter    = metrics.NewMeter("paa/req/blocks/in/traffic")
		reqBlockOutPacketsMeter   = metrics.NewMeter("paa/req/blocks/out/packets")
		reqBlockOutTrafficMeter   = metrics.NewMeter("paa/req/blocks/out/traffic")
		reqHeaderInPacketsMeter   = metrics.NewMeter("paa/req/headers/in/packets")
		reqHeaderInTrafficMeter   = metrics.NewMeter("paa/req/headers/in/traffic")
		reqHeaderOutPacketsMeter  = metrics.NewMeter("paa/req/headers/out/packets")
		reqHeaderOutTrafficMeter  = metrics.NewMeter("paa/req/headers/out/traffic")
		reqBodyInPacketsMeter     = metrics.NewMeter("paa/req/bodies/in/packets")
		reqBodyInTrafficMeter     = metrics.NewMeter("paa/req/bodies/in/traffic")
		reqBodyOutPacketsMeter    = metrics.NewMeter("paa/req/bodies/out/packets")
		reqBodyOutTrafficMeter    = metrics.NewMeter("paa/req/bodies/out/traffic")
		reqStateInPacketsMeter    = metrics.NewMeter("paa/req/states/in/packets")
		reqStateInTrafficMeter    = metrics.NewMeter("paa/req/states/in/traffic")
		reqStateOutPacketsMeter   = metrics.NewMeter("paa/req/states/out/packets")
		reqStateOutTrafficMeter   = metrics.NewMeter("paa/req/states/out/traffic")
		reqReceiptInPacketsMeter  = metrics.NewMeter("paa/req/receipts/in/packets")
		reqReceiptInTrafficMeter  = metrics.NewMeter("paa/req/receipts/in/traffic")
		reqReceiptOutPacketsMeter = metrics.NewMeter("paa/req/receipts/out/packets")
		reqReceiptOutTrafficMeter = metrics.NewMeter("paa/req/receipts/out/traffic")*/
	miscInPacketsMeter  = metrics.NewRegisteredMeter("les/misc/in/packets", nil)
	miscInTrafficMeter  = metrics.NewRegisteredMeter("les/misc/in/traffic", nil)
	miscOutPacketsMeter = metrics.NewRegisteredMeter("les/misc/out/packets", nil)
	miscOutTrafficMeter = metrics.NewRegisteredMeter("les/misc/out/traffic", nil)
)

// meteredMsgReadWriter is a wrapper around a p2p.MsgReadWriter, capable of
// accumulating the above defined metrics based on the data stream contents.
type meteredMsgReadWriter struct {
	p2p.MsgReadWriter     // Wrapped message stream to meter
	version           int // Protocol version to select correct meters
}

// newMeteredMsgWriter wraps a p2p MsgReadWriter with metering support. If the
// metrics system is disabled, this function returns the original object.
func newMeteredMsgWriter(rw p2p.MsgReadWriter) p2p.MsgReadWriter {
	if !metrics.Enabled {
		return rw
	}
	return &meteredMsgReadWriter{MsgReadWriter: rw}
}

// Init sets the protocol version used by the stream to know which meters to
// increment in case of overlapping message ids between protocol versions.
func (rw *meteredMsgReadWriter) Init(version int) {
	rw.version = version
}

func (rw *meteredMsgReadWriter) ReadMsg() (p2p.Msg, error) {
	// Read the message and short circuit in case of an error
	msg, err := rw.MsgReadWriter.ReadMsg()
	if err != nil {
		return msg, err
	}
	// Account for the data traffic
	packets, traffic := miscInPacketsMeter, miscInTrafficMeter
	packets.Mark(1)
	traffic.Mark(int64(msg.Size))

	return msg, err
}

func (rw *meteredMsgReadWriter) WriteMsg(msg p2p.Msg) error {
	// Account for the data traffic
	packets, traffic := miscOutPacketsMeter, miscOutTrafficMeter
	packets.Mark(1)
	traffic.Mark(int64(msg.Size))

	// Send the packet to the p2p layer
	return rw.MsgReadWriter.WriteMsg(msg)
}
