# Análisis de jOOQ y propuesta de equivalente en Go

## 1. Síntesis de jOOQ — la sintaxis de queries

### 1.1 Dos puntos de entrada

jOOQ ofrece dos APIs paralelas que comparten el mismo árbol fluente:

- **`DSL` (estático)** — construye queries "desadjuntas" (sin conexión):
  `DSL.select(BOOK.TITLE).from(BOOK).where(BOOK.ID.eq(1))`
- **`DSLContext` (contextual)** — el mismo árbol, pero ligado a `Configuration` (JDBC + dialecto + settings); permite ejecutar inmediatamente:
  `create.selectFrom(BOOK).where(...).fetch()`

Archivos clave: `.tmp/jOOQ/jOOQ/src/main/java/org/jooq/impl/DSL.java`, `.../DSLContext.java`.

### 1.2 El patrón "step interfaces" — el corazón del diseño

Cada cláusula devuelve un **tipo de interfaz distinto** que representa los siguientes pasos legales. Esto convierte el orden de las cláusulas SQL en un problema de tipos: el compilador rechaza secuencias inválidas.

Cadena completa de SELECT:

```
DSL.select(...)               -> SelectSelectStep<RecordN<...>>
       .from(table)           -> SelectJoinStep<R>     (extiende SelectWhereStep)
       .join(t2).on(cond)     -> SelectOnConditionStep<R>
       .where(cond)           -> SelectConditionStep<R>
       .and(cond) / .or(...)  -> SelectConditionStep<R>
       .groupBy(...)          -> SelectHavingStep<R>
       .having(cond)          -> SelectHavingStep<R>
       .orderBy(field.asc())  -> SelectLimitStep<R>
       .limit(10).offset(20)  -> SelectFinalStep<R>    (extiende Select<R> -> ResultQuery<R>)
       .fetch()               -> Result<R>
```

Lo importante: `SelectWhereStep` extiende `SelectGroupByStep` que extiende `SelectOrderByStep`... por lo que **se pueden omitir cláusulas opcionales**, pero **no se pueden poner en orden incorrecto**. INSERT/UPDATE/DELETE siguen el mismo patrón (`InsertSetStep`, `InsertValuesStepN`, `UpdateSetStep`, `UpdateConditionStep`, `DeleteWhereStep`).

### 1.3 Tipado paramétrico de columnas y filas

```java
public interface Field<T> extends SelectField<T>, ... {
    Condition eq(T value);        // Field<Integer>.eq(Integer)
    Condition gt(T value);
    Condition in(T... values);
    Condition like(String pattern);   // sólo en Field<String> implícitamente
    Field<T> as(String alias);
}

public interface TableField<R extends Record, T> extends Field<T> { ... }
```

`Record1<T1>` ... `Record22<T1..T22>` preservan los tipos por posición. Así:

```java
Result<Record2<String, Integer>> r =
    DSL.select(BOOK.TITLE, BOOK.PRICE).from(BOOK).fetch();
for (Record2<String, Integer> row : r) {
    String title = row.value1();   // tipo correcto en compilación
    Integer price = row.value2();
}
```

### 1.4 Condiciones componibles

`Condition` **es a su vez** `Field<Boolean>`, por lo que se puede usar dentro de un SELECT, AND/OR/NOT, y guardar en variables:

```java
Condition c = BOOK.TITLE.eq("foo").and(BOOK.PRICE.gt(10));
selectFrom(BOOK).where(c).fetch();
```

### 1.5 Código generado

Por cada tabla, el generador produce un singleton con campos `public final TableField<Record, T>` por columna:

```java
public class Book extends TableImpl<Record> {
    public static final Book BOOK = new Book();
    public final TableField<Record, Integer> ID    = createField(...);
    public final TableField<Record, String>  TITLE = createField(...);
    public final TableField<Record, Double>  PRICE = createField(...);
    @Override public Book as(String alias) { ... }   // soporte para JOINs
}
```

Con import estático: `selectFrom(BOOK).where(BOOK.PRICE.gt(10.0))`.

### 1.6 Ejecución

`ResultQuery<R>` (en `org/jooq/ResultQuery.java`) expone `fetch()`, `fetchOne()`, `fetchSingle()`, `fetchStream()`, `fetchLazy()`, `fetch(Class<E>)`, `fetchInto(...)`. La traducción a dialecto ocurre en ejecución a partir del AST.

---

## 2. Estado del arte en Go

Resumen del relevamiento:

| Librería   | Schema        | Columnas        | Tipado fila     | Step ints. | Dinámico |
|------------|---------------|-----------------|-----------------|------------|----------|
| **sqlc**   | SQL → codegen | n/a (SQL crudo) | structs gen.    | n/a        | **No**   |
| **ent**    | code-first    | predicados gen. | structs gen.    | grafo, no SQL | parcial |
| **squirrel** | ninguno     | `string`        | `interface{}`   | no         | sí       |
| **go-jet** | DB → codegen  | `ColumnString/Integer/...` (categoría) | reflexión sobre struct | **no** | sí |
| **bun**    | tags struct   | `string` en `?`-fragments | struct | no | sí |
| **gorm**   | tags struct   | `string` SQL crudo | struct | no | sí |

**go-jet** es lo más cercano a jOOQ, pero le faltan: (a) step interfaces que prohíban órdenes inválidos en tiempo de compilación, (b) tipos `RecordN[T1..Tn]` para proyecciones ad-hoc, (c) `Field[T]` paramétrico real (su tipado es por categoría, no por tipo Go), (d) traducción a dialecto en runtime — el dialecto se elige al generar.

**El hueco real**: no existe en Go una librería que combine codegen desde DB en vivo + `Field[T]` con generics + `RecordN[T1..Tn]` + step interfaces + traducción a dialecto. Go 1.18+ (generics) hace ese diseño viable hoy de un modo que no lo era cuando se escribió go-jet.

---

## 3. Cómo implementarlo en Go

### 3.1 Step interfaces con generics

Cada paso es una interfaz parametrizada en el tipo-fila, y cada método devuelve la siguiente interfaz:

```go
type SelectFromStep[R any] interface {
    From(t Table) SelectJoinStep[R]
}
type SelectJoinStep[R any] interface {
    SelectWhereStep[R]                       // permite saltar JOIN
    Join(t Table) SelectOnStep[R]
    LeftJoin(t Table) SelectOnStep[R]
}
type SelectOnStep[R any] interface {
    On(c Condition) SelectJoinStep[R]
}
type SelectWhereStep[R any] interface {
    SelectGroupByStep[R]                     // opcional: heredar saltos
    Where(c Condition) SelectConditionStep[R]
}
type SelectConditionStep[R any] interface {
    SelectGroupByStep[R]
    And(c Condition) SelectConditionStep[R]
    Or(c Condition)  SelectConditionStep[R]
}
type SelectGroupByStep[R any] interface {
    SelectOrderByStep[R]
    GroupBy(fields ...AnyField) SelectHavingStep[R]
}
type SelectOrderByStep[R any] interface {
    SelectLimitStep[R]
    OrderBy(orders ...OrderField) SelectLimitStep[R]
}
type SelectLimitStep[R any] interface {
    SelectFinalStep[R]
    Limit(n int) SelectLimitStep[R]
    Offset(n int) SelectLimitStep[R]
}
type SelectFinalStep[R any] interface {
    Fetch(ctx context.Context, db Querier)    ([]R, error)
    FetchOne(ctx context.Context, db Querier) (R, error)
    SQL() (string, []any, error)
}
```

La "herencia" de jOOQ entre steps se modela vía **interfaces embebidas** (`SelectGroupByStep[R]` embebida en `SelectWhereStep[R]`), lo que permite saltar cláusulas opcionales pero no romper el orden.

### 3.2 `Field[T]` paramétrico

```go
type Field[T any] interface {
    Name() string
    EQ(T)  Condition
    NE(T)  Condition
    GT(T)  Condition
    LT(T)  Condition
    GE(T)  Condition
    LE(T)  Condition
    In(...T) Condition
    IsNull() Condition
    As(alias string) Field[T]
}

// Métodos específicos por tipo: sólo Field[string] tiene Like, etc.
type StringField interface {
    Field[string]
    Like(pattern string) Condition
}
```

`Condition` es a su vez `Field[bool]`, replicando la decisión de jOOQ:

```go
type Condition interface {
    Field[bool]
    And(Condition) Condition
    Or(Condition) Condition
    Not() Condition
}
```

### 3.3 `RecordN[T1..Tn]` para proyecciones tipadas

Go no soporta varargs genéricos, así que se replica el truco de jOOQ generando `Record1` … `Record22`:

```go
type Record2[T1, T2 any] struct { V1 T1; V2 T2 }

func Select2[T1, T2 any](
    f1 Field[T1], f2 Field[T2],
) SelectFromStep[Record2[T1, T2]] { ... }

// Uso:
rows, _ := db.Select2(book.Title, book.Price).
    From(book.Table).
    Where(book.Price.GT(10.0)).
    OrderBy(book.Title.Asc()).
    Limit(20).
    Fetch(ctx)
for _, r := range rows {
    fmt.Println(r.V1, r.V2)   // V1 es string, V2 es float64
}
```

Para proyecciones más anchas que 22 columnas o dinámicas, fallback a `Record map[string]any` o a un destino struct con `FetchInto(&dst)` por reflexión, igual que jOOQ.

### 3.4 Tablas generadas — codegen desde DB en vivo

Un comando `jooq-go gen --dsn postgres://... -o internal/db` introspecta `information_schema` y emite:

```go
// internal/db/book.gen.go
package db

type bookTable struct {
    table
    ID    Field[int64]
    Title Field[string]
    Price Field[float64]
}
var Book = &bookTable{
    table: newTable("book"),
    ID:    newField[int64]("id"),
    Title: newField[string]("title"),
    Price: newField[float64]("price"),
}
func (b *bookTable) As(alias string) *bookTable { ... }
```

Cada columna conoce su tipo SQL real, por lo que el generator puede decidir si emitir `Field[time.Time]`, `Field[uuid.UUID]`, `Field[pgtype.JSONB]`, etc.

### 3.5 Capa de ejecución y dialecto

```go
type Querier interface {       // compatible con database/sql y pgx
    QueryContext(ctx context.Context, sql string, args ...any) (*sql.Rows, error)
    ExecContext (ctx context.Context, sql string, args ...any) (sql.Result, error)
}

type DSL struct {
    db      Querier
    dialect Dialect           // postgres, mysql, sqlite, mssql, oracle...
}
func (d *DSL) Select2[T1, T2 any](f1 Field[T1], f2 Field[T2]) SelectFromStep[Record2[T1, T2]] { ... }
```

Los builders construyen un **AST** (`selectNode`, `joinNode`, `predicateNode`), y un `renderer` por dialecto traduce a SQL en `SQL()`. Así, igual que jOOQ, una sola query Go puede emitir `LIMIT n OFFSET m` o `OFFSET m ROWS FETCH NEXT n ROWS ONLY` según destino.

### 3.6 Limitaciones honestas que conviene asumir

- **Limitación de Go en métodos genéricos**: los métodos sobre receptor no admiten parámetros de tipo propios. Por eso `Select2`, `Select3`, ..., `Select22` deben ser **funciones top-level genéricas** (o métodos sobre `*DSL` que sí pueden serlo si se piensan como funciones que toman el contexto). jOOQ resuelve esto con overloading; en Go hay que generar `Select1`..`SelectN`.
- **Sin overloading**: tipos específicos como `StringField` con `Like`, `NumericField` con `Add/Mul`, etc., requieren interfaces dedicadas o un set de helpers (`Like(field, pattern)`).
- **Generics y reflexión para `FetchInto`**: mapeo a structs externos seguirá necesitando reflexión, idéntico a jOOQ con `into(Class)`.

---

## 4. Hoja de ruta sugerida

1. **Núcleo AST + renderer** para PostgreSQL (un solo dialecto al principio).
2. **`Field[T]` + `Condition`** con operadores básicos (`eq, ne, gt, lt, in, isNull, like`).
3. **Step interfaces para SELECT** con `Record1`..`Record5` como prueba; expandir a 22 con go generate.
4. **INSERT/UPDATE/DELETE** con sus propias cadenas.
5. **Codegen desde DB**: introspección de `information_schema` → archivos `*.gen.go` con tablas/columnas tipadas.
6. **Segundo dialecto** (MySQL o SQLite) para validar la separación AST/render.
7. **Funciones avanzadas**: CTEs, window functions, `RETURNING`, `UPSERT`/`MERGE`.

El punto crítico en el que esta librería supera a go-jet desde el primer día es el **par step-interfaces + `Field[T]` con generics + `RecordN[T1..Tn]`**: ninguna librería Go los combina hoy, y son exactamente la razón por la que jOOQ se siente como SQL tipado en lugar de "strings con autocompletado".
