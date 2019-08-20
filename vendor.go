package irain

import (
	"fmt"
	"github.com/nextabc-lab/edgex-go/extra"
)

//
// Author: 陈哈哈 chenyongjia@parkingwang.com, yoojiachen@gmail.com
//

const (
	VendorName = "IRAIN"
	DriverName = "TCP"
)

////

func directName(dir byte) string {
	if extra.DirectIn == dir {
		return "IN"
	} else {
		return "OUT"
	}
}

func makeGroupId(ctrlId byte) string {
	return fmt.Sprintf("SNID[%d]", ctrlId)
}

func makeMajorId(doorId int) string {
	return fmt.Sprintf("DOOR[%d]", doorId)
}
