package main

import (
    "fmt"

    yasdb "cod-git.sics.com/cod-noah/gorm-yasdb"
    "gorm.io/gorm"
)

type Abc struct {
    gorm.Model
    Status string
    Role   string
    Point  int
}

func main() {
    dsn := "sys/sys@192.168.30.220:1688"
    db, err := gorm.Open(yasdb.Open(dsn), &gorm.Config{})
    if err != nil {
        panic(err)
    }

    db.Exec("drop table if exists abcs")
    db.Exec("drop sequence abcs_seq")

    db.Exec("create sequence abcs_seq start with 1 increment by 1")
    db.Debug().AutoMigrate(&Abc{})

    err = db.Transaction(func(tx *gorm.DB) error {
        a := &[]Abc{
            {Status: "red", Role: "light", Point: 1},
            {Status: "yellow", Role: "light", Point: 2},
            {Status: "green", Role: "light", Point: 3},
        }
        return tx.Debug().Create(&a).Error
    })
    if err != nil {
        panic(err)
    }

    b := []*Abc{}
    err = db.Debug().Where("point in (?)", []int{1, 2, 3}).Limit(1).Offset(1).Find(&b).Error
    if err != nil {
        panic(err)
    }

    fmt.Printf("%+v", b[0])
}
