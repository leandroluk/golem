package postgres

import "github.com/leandroluk/golem/core"

func f(t core.FieldToken, v any) core.FieldOption { return core.FieldOption{Token: t, Value: v} }

// ---------- Numéricos ----------

const (
	IntO             core.FieldToken = "postgres.int"
	Int2O            core.FieldToken = "postgres.int2"
	Int4O            core.FieldToken = "postgres.int4"
	Int8O            core.FieldToken = "postgres.int8"
	SmallIntO        core.FieldToken = "postgres.smallint"
	BigIntO          core.FieldToken = "postgres.bigint"
	DecimalO         core.FieldToken = "postgres.decimal"
	NumericO         core.FieldToken = "postgres.numeric"
	RealO            core.FieldToken = "postgres.real"
	DoublePrecisionO core.FieldToken = "postgres.double_precision"
	MoneyO           core.FieldToken = "postgres.money"
)

func Int() core.FieldOption                         { return f(IntO, nil) }
func Int2() core.FieldOption                        { return f(Int2O, nil) }
func Int4() core.FieldOption                        { return f(Int4O, nil) }
func Int8() core.FieldOption                        { return f(Int8O, nil) }
func SmallInt() core.FieldOption                    { return f(SmallIntO, nil) }
func BigInt() core.FieldOption                      { return f(BigIntO, nil) }
func Decimal(precision, scale int) core.FieldOption { return f(DecimalO, [2]int{precision, scale}) }
func Numeric(precision, scale int) core.FieldOption { return f(NumericO, [2]int{precision, scale}) }
func Real() core.FieldOption                        { return f(RealO, nil) }
func DoublePrecision() core.FieldOption             { return f(DoublePrecisionO, nil) }
func Money() core.FieldOption                       { return f(MoneyO, nil) }

// ---------- Datas/Horas ----------

const (
	DateO        core.FieldToken = "postgres.date"
	TimeO        core.FieldToken = "postgres.time"
	TimeTZO      core.FieldToken = "postgres.timetz"
	TimestampO   core.FieldToken = "postgres.timestamp"
	TimestamptzO core.FieldToken = "postgres.timestamptz"
	IntervalO    core.FieldToken = "postgres.interval"
)

func Date() core.FieldOption { return f(DateO, nil) }

func Time(precision ...int) core.FieldOption {
	if len(precision) > 0 {
		return f(TimeO, precision[0])
	}
	return f(TimeO, nil)
}

func TimeTZ(precision ...int) core.FieldOption {
	if len(precision) > 0 {
		return f(TimeTZO, precision[0])
	}
	return f(TimeTZO, nil)
}

func Timestamp(precision ...int) core.FieldOption {
	if len(precision) > 0 {
		return f(TimestampO, precision[0])
	}
	return f(TimestampO, nil)
}

func Timestamptz(precision ...int) core.FieldOption {
	if len(precision) > 0 {
		return f(TimestamptzO, precision[0])
	}
	return f(TimestamptzO, nil)
}

func Interval() core.FieldOption { return f(IntervalO, nil) }

// ---------- Texto ----------

const (
	VarCharO core.FieldToken = "postgres.varchar"
	CharO    core.FieldToken = "postgres.char"
	TextO    core.FieldToken = "postgres.text"
	CitextO  core.FieldToken = "postgres.citext"
)

func VarChar(length int) core.FieldOption { return f(VarCharO, length) }
func Char(length int) core.FieldOption    { return f(CharO, length) }
func Text() core.FieldOption              { return f(TextO, nil) }
func Citext() core.FieldOption            { return f(CitextO, nil) }

// ---------- Binário ----------

const (
	ByteaO core.FieldToken = "postgres.bytea"
)

func Bytea() core.FieldOption { return f(ByteaO, nil) }

// ---------- Estruturados/Especiais ----------

const (
	JSONO   core.FieldToken = "postgres.json"
	JSONBO  core.FieldToken = "postgres.jsonb"
	UUIDO   core.FieldToken = "postgres.uuid"
	ArrayO  core.FieldToken = "postgres.array"
	EnumO   core.FieldToken = "postgres.enum"
	VectorO core.FieldToken = "postgres.vector"
)

func JSON() core.FieldOption                               { return f(JSONO, nil) }
func JSONB() core.FieldOption                              { return f(JSONBO, nil) }
func UUID() core.FieldOption                               { return f(UUIDO, nil) }
func Array[T any](elem *core.FieldOption) core.FieldOption { return f(ArrayO, elem) }

func Enum(values ...string) core.FieldOption { return f(EnumO, append([]string(nil), values...)) }

func Vector(dim int) core.FieldOption { return f(VectorO, dim) }

// ---------- OS / Outros ----------

const (
	TSVectorO core.FieldToken = "postgres.tsvector"
	TSQueryO  core.FieldToken = "postgres.tsquery"
	HstoreO   core.FieldToken = "postgres.hstore"
	LtreeO    core.FieldToken = "postgres.ltree"
	CubeO     core.FieldToken = "postgres.cube"
)

func TSVector() core.FieldOption { return f(TSVectorO, nil) }
func TSQuery() core.FieldOption  { return f(TSQueryO, nil) }
func Hstore() core.FieldOption   { return f(HstoreO, nil) }
func Ltree() core.FieldOption    { return f(LtreeO, nil) }
func Cube() core.FieldOption     { return f(CubeO, nil) }

// ---------- Geo (PostGIS) ----------

const (
	PointO              core.FieldToken = "postgres.point"
	LineO               core.FieldToken = "postgres.line"
	LsegO               core.FieldToken = "postgres.lseg"
	BoxO                core.FieldToken = "postgres.box"
	CircleO             core.FieldToken = "postgres.circle"
	PathO               core.FieldToken = "postgres.path"
	PolygonO            core.FieldToken = "postgres.polygon"
	LineStringO         core.FieldToken = "postgres.linestring"
	MultiPointO         core.FieldToken = "postgres.multipoint"
	MultiLineStringO    core.FieldToken = "postgres.multilinestring"
	MultiPolygonO       core.FieldToken = "postgres.multipolygon"
	GeometryO           core.FieldToken = "postgres.geometry"
	GeographyO          core.FieldToken = "postgres.geography"
	GeometryCollectionO core.FieldToken = "postgres.geometrycollection"
)

func Point() core.FieldOption              { return f(PointO, nil) }
func Line() core.FieldOption               { return f(LineO, nil) }
func Lseg() core.FieldOption               { return f(LsegO, nil) }
func Box() core.FieldOption                { return f(BoxO, nil) }
func Circle() core.FieldOption             { return f(CircleO, nil) }
func Path() core.FieldOption               { return f(PathO, nil) }
func Polygon() core.FieldOption            { return f(PolygonO, nil) }
func LineString() core.FieldOption         { return f(LineStringO, nil) }
func MultiPoint() core.FieldOption         { return f(MultiPointO, nil) }
func MultiLineString() core.FieldOption    { return f(MultiLineStringO, nil) }
func MultiPolygon() core.FieldOption       { return f(MultiPolygonO, nil) }
func Geometry() core.FieldOption           { return f(GeometryO, nil) }
func Geography() core.FieldOption          { return f(GeographyO, nil) }
func GeometryCollection() core.FieldOption { return f(GeometryCollectionO, nil) }

// ---------- Ranges ----------

const (
	Int4RangeO      core.FieldToken = "postgres.int4range"
	Int8RangeO      core.FieldToken = "postgres.int8range"
	NumRangeO       core.FieldToken = "postgres.numrange"
	TSRangeO        core.FieldToken = "postgres.tsrange"
	TSTZRangeO      core.FieldToken = "postgres.tstzrange"
	DateRangeO      core.FieldToken = "postgres.daterange"
	Int4MultiRangeO core.FieldToken = "postgres.int4multirange"
	Int8MultiRangeO core.FieldToken = "postgres.int8multirange"
	NumMultiRangeO  core.FieldToken = "postgres.nummultirange"
	TSMultiRangeO   core.FieldToken = "postgres.tsmultirange"
	TSTZMultiRangeO core.FieldToken = "postgres.tstzmultirange"
	DateMultiRangeO core.FieldToken = "postgres.datemultirange"
)

func Int4Range() core.FieldOption      { return f(Int4RangeO, nil) }
func Int8Range() core.FieldOption      { return f(Int8RangeO, nil) }
func NumRange() core.FieldOption       { return f(NumRangeO, nil) }
func TSRange() core.FieldOption        { return f(TSRangeO, nil) }
func TSTZRange() core.FieldOption      { return f(TSTZRangeO, nil) }
func DateRange() core.FieldOption      { return f(DateRangeO, nil) }
func Int4MultiRange() core.FieldOption { return f(Int4MultiRangeO, nil) }
func Int8MultiRange() core.FieldOption { return f(Int8MultiRangeO, nil) }
func NumMultiRange() core.FieldOption  { return f(NumMultiRangeO, nil) }
func TSMultiRange() core.FieldOption   { return f(TSMultiRangeO, nil) }
func TSTZMultiRange() core.FieldOption { return f(TSTZMultiRangeO, nil) }
func DateMultiRange() core.FieldOption { return f(DateMultiRangeO, nil) }

// ---------- Redes ----------

const (
	CIDRO     core.FieldToken = "postgres.cidr"
	INETO     core.FieldToken = "postgres.inet"
	MACAddrO  core.FieldToken = "postgres.macaddr"
	MACAddr8O core.FieldToken = "postgres.macaddr8"
)

func CIDR() core.FieldOption     { return f(CIDRO, nil) }
func INET() core.FieldOption     { return f(INETO, nil) }
func MACAddr() core.FieldOption  { return f(MACAddrO, nil) }
func MACAddr8() core.FieldOption { return f(MACAddr8O, nil) }

// ---------- Boolean ----------

const (
	BooleanO core.FieldToken = "postgres.boolean"
)

func Boolean() core.FieldOption { return f(BooleanO, nil) }
