# csv

使用结构体序列化和反序列化 csv 格式数据

### 使用

```go
type QTime time.Time

func (q QTime) MarshalCSV() (string, error) {
	return (time.Time)(q).Format("2006-01-02 15:04"), nil
}

var _ csv.Marshaler = QTime{}

func (q *QTime) UnmarshalCSV(s string) error {
	t, err := time.Parse("2006/1/2 15:04", s)
	if err != nil {
		return err
	}
	*q = QTime(t)
	return nil
}

var _ csv.Unmarshaler = (*QTime)(nil)

type Tags []string

func (t Tags) MarshalCSV() (string, error) {
	if len(t) == 0 {
		return "", nil
	} else if len(t) == 1 {
		return t[0], nil
	}
	return "#" + strings.Join(t, "#"), nil
}

var _ csv.Marshaler = Tags{}

func (t *Tags) UnmarshalCSV(s string) error {
	var parts Tags
	for _, v := range strings.Split(s, "#") {
		if v != "" {
			parts = append(parts, v)
		}
	}
	*t = parts
	return nil
}

var _ csv.Unmarshaler = (*Tags)(nil)

func (t Tags) String() string {
	return strings.Join(t, "/")
}

type Image url.URL

func (i Image) MarshalCSV() (string, error) {
	return (*url.URL)(&i).String(), nil
}

var _ csv.Marshaler = Image{}

func (i *Image) UnmarshalCSV(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	*i = Image(*u)
	return nil
}

var _ csv.Unmarshaler = (*Image)(nil)

type Bill struct {
	A1  *QTime  `csv:"时间"`
	A4  float64 `csv:"金额"`
	A2  string  `csv:"分类"`
	A3  string  `csv:"类型"`
	A5  string  `csv:"账户1"`
	A6  string  `csv:"账户2"`
	A7  string  `csv:"备注"`
	A8  string  `csv:"账单标记"`
	A9  string  `csv:"手续费"`
	A10 string  `csv:"优惠券"`
	A11 *Tags   `csv:"标签"`
	A12 *Image  `csv:"账单图片"`
}

type OrderedBill Bill

func (OrderedBill) CSVHeader() []string {
	return []string{"时间", "类型", "金额", "账单图片"}
}

var _ csv.Header = OrderedBill{}

var data = []byte(`时间,分类,类型,金额,账户1,账户2,备注,账单标记,手续费,优惠券,标签,账单图片
2022/7/12 21:20,三餐,支出,28.58,微信,,去肯德基吃汉堡（此行数据是示例，可以删除）,不计收支,,,种草,http://billimg.qianjiapp.com/202006300908267611e24759b6c1c8781361af861!webporigin
2022/7/8 22:15,工资,收入,1000,,,7月份工资（此行数据是示例，可以删除）,,,,#老婆#旅行,
2022/7/8 10:10,,转账,200,支付宝,招商银行卡,支付宝提现1000元到银行卡（此行数据是示例，可以删除）,,,,#冲动消费#拔草,
2022/7/8 22:15,奶茶,支出,12.5,,,茶百道（此行数据是示例，可以删除）,不计收支&预算,,,,`)

var r1 = `时间,金额,分类,类型,账户1,账户2,备注,账单标记,手续费,优惠券,标签,账单图片
2022-07-12 21:20,28.58,三餐,支出,微信,,去肯德基吃汉堡（此行数据是示例，可以删除）,不计收支,,,种草,http://billimg.qianjiapp.com/202006300908267611e24759b6c1c8781361af861!webporigin
2022-07-08 22:15,1000,工资,收入,,,7月份工资（此行数据是示例，可以删除）,,,,#老婆#旅行,
2022-07-08 10:10,200,,转账,支付宝,招商银行卡,支付宝提现1000元到银行卡（此行数据是示例，可以删除）,,,,#冲动消费#拔草,
2022-07-08 22:15,12.5,奶茶,支出,,,茶百道（此行数据是示例，可以删除）,不计收支&预算,,,,
`

var r2 = `时间/类型/金额/账单图片
2022-07-12 21:20/支出/28.58/"http://billimg.qianjiapp.com/202006300908267611e24759b6c1c8781361af861!webporigin"
2022-07-08 22:15/收入/1000/
2022-07-08 10:10/转账/200/
2022-07-08 22:15/支出/12.5/
`

func TestMarshal(t *testing.T) {
	records, err := csv.Unmarshal[Bill](data)
	if err != nil {
		t.Fatal(err)
	}
	s, err := csv.MarshalText(records)
	if err != nil {
		t.Fatal(err)
	}
	if s != r1 {
		t.Fatal(s)
	}
	// csv.Header
	ordered := make([]OrderedBill, 0, len(records))
	for _, r := range records {
		ordered = append(ordered, OrderedBill(r))
	}
	// csv.WriterHandler
	s, err = csv.MarshalText(ordered, csv.WriterComma('/'))
	if err != nil {
		t.Fatal(err)
	}
	if s != r2 {
		t.Fatal(s)
	}
}
```
