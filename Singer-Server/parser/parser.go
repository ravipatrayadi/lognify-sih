package parser

import (
	"math"
	"sync"
	"time"

	"github.com/housepower/clickhouse_sinker/model"
	"github.com/housepower/clickhouse_sinker/util"
	"github.com/thanos-io/thanos/pkg/errors"
	"github.com/valyala/fastjson"
)

var (
	Layouts = []string{
		"2006-01-02 15:04:05Z0700",
		"2006-01-02 15:04:05",
		"Jan 02, 2006 15:04:05Z0700",
		"Jan 02, 2006 15:04:05",
		"02/Jan/2006 15:04:05 Z07:00",
		"02/Jan/2006 15:04:05 Z0700",
		"02/Jan/2006 15:04:05",
		"2006-01-02",
		"02/01/2006",
		"02/Jan/2006",
		"Jan 02, 2006",
		"Mon Jan 02, 2006",
	}
	Epoch            = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	ErrParseDateTime = errors.Newf("value doesn't contain DateTime")
)

type Parser interface {
	Parse(bs []byte) (metric model.Metric, err error)
}

type Pool struct {
	name         string
	csvFormat    map[string]int
	delimiter    string
	timeZone     *time.Location
	timeUnit     float64
	knownLayouts sync.Map
	pool         sync.Pool
	once         sync.Once
	fields       string
}

func NewParserPool(name string, csvFormat []string, delimiter string, timezone string, timeunit float64, fields string) (pp *Pool, err error) {
	var tz *time.Location
	if timezone == "" {
		tz = time.Local
	} else if tz, err = time.LoadLocation(timezone); err != nil {
		err = errors.Wrapf(err, "")
		return
	}
	pp = &Pool{
		name:      name,
		delimiter: delimiter,
		timeZone:  tz,
		timeUnit:  timeunit,
		fields:    fields,
	}
	if csvFormat != nil {
		pp.csvFormat = make(map[string]int, len(csvFormat))
		for i, title := range csvFormat {
			pp.csvFormat[title] = i
		}
	}
	return
}

func (pp *Pool) Get() (Parser, error) {
	v := pp.pool.Get()
	if v == nil {
		switch pp.name {
		case "gjson":
			return &GjsonParser{pp: pp}, nil
		case "csv":
			if pp.fields != "" {
				util.Logger.Warn("extra fields for csv parser is not supported, fields ignored")
			}
			return &CsvParser{pp: pp}, nil
		case "fastjson":
			fallthrough
		default:
			var obj *fastjson.Object
			if pp.fields != "" {
				value, err := fastjson.Parse(pp.fields)
				if err != nil {
					err = errors.Wrapf(err, "failed to parse fields as a valid json object")
					return nil, err
				}
				obj, err = value.Object()
				if err != nil {
					err = errors.Wrapf(err, "failed to retrive fields member")
					return nil, err
				}
			}
			return &FastjsonParser{pp: pp, fields: obj}, nil
		}
	}
	return v.(Parser), nil
}

func (pp *Pool) Put(p Parser) {
	pp.pool.Put(p)
}

func (pp *Pool) ParseDateTime(key string, val string) (t time.Time, err error) {
	var layout string
	var lay interface{}
	var ok bool
	var t2 time.Time
	if val == "" {
		err = ErrParseDateTime
		return
	}
	if lay, ok = pp.knownLayouts.Load(key); !ok {
		t2, layout = parseInLocation(val, pp.timeZone)
		if layout == "" {
			err = ErrParseDateTime
			return
		}
		t = t2
		pp.knownLayouts.Store(key, layout)
		return
	}
	if layout, ok = lay.(string); !ok {
		err = ErrParseDateTime
		return
	}
	if t2, err = time.ParseInLocation(layout, val, pp.timeZone); err != nil {
		err = ErrParseDateTime
		return
	}
	t = t2.UTC()
	return
}

func parseInLocation(val string, loc *time.Location) (t time.Time, layout string) {
	var err error
	var lay string
	for _, lay = range Layouts {
		if t, err = time.ParseInLocation(lay, val, loc); err == nil {
			t = t.UTC()
			layout = lay
			return
		}
	}
	return
}

func UnixFloat(sec, unit float64) (t time.Time) {
	sec *= unit
	if sec < 0 || sec >= 4294967296.0 {
		return Epoch
	}
	i, f := math.Modf(sec)
	return time.Unix(int64(i), int64(f*1e9)).UTC()
}
