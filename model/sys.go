package model

import "time"

type SysAsset struct {
	ID      uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Chain   string    `gorm:"column:chain" json:"chain"`
	Ca      string    `gorm:"column:ca" json:"ca"`
	Name    string    `gorm:"column:name" json:"name"`
	Symbol  string    `gorm:"column:symbol" json:"symbol"`
	Type    string    `gorm:"column:type" json:"type"`
	AddTime time.Time `gorm:"column:add_time" json:"add_time"`
}

func (SysAsset) TableName() string {
	return TB_SYS_ASSET
}
