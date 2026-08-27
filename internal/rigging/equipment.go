package rigging

import "time"

func EquipmentUsable(e Equipment, now time.Time) bool { return ValidateEquipment(e, now) == nil }
