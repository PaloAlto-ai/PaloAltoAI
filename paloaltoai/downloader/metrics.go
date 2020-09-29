// Copyright 2015 The go-PaloAltoAi Authors
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

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/PaloAltoAi/go-PaloAltoAi/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("paa/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("paa/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("paa/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("paa/downloader/headers/timeout", nil)

	bodyInMeter      = metrics.NewRegisteredMeter("paa/downloader/bodies/in", nil)
	bodyReqTimer     = metrics.NewRegisteredTimer("paa/downloader/bodies/req", nil)
	bodyDropMeter    = metrics.NewRegisteredMeter("paa/downloader/bodies/drop", nil)
	bodyTimeoutMeter = metrics.NewRegisteredMeter("paa/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("paa/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("paa/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("paa/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("paa/downloader/receipts/timeout", nil)

	stateInMeter   = metrics.NewRegisteredMeter("paa/downloader/states/in", nil)
	stateDropMeter = metrics.NewRegisteredMeter("paa/downloader/states/drop", nil)
)
