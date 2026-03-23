# mtail插件

## 简介
功能：提取日志内容，转换为监控metrics

+ 输入： 日志
+ 输出： metrics 按照mtail语法输出, 仅支持counter、gauge、histogram
+ 处理： 本质是golang的正则提取+表达式计算

## 启动
编辑mtail.toml文件, 一般每个instance需要指定不同的progs参数（不同的progs文件或者目录）,否则指标会相互干扰。
**注意**: 如果不同instance使用相同progs, 可以通过给每个instance增加labels做区分，
```toml
labels = { k1=v1 }
```
或
```toml
[instances.labels]
k1=v1
```

1. conf/inputs.mtail/mtail.toml中指定instance
```toml

[[instances]]
## 指定mtail prog的目录
progs = "/path/to/prog1"
## 指定mtail要读取的日志
logs = ["/path/to/a.log", "path/to/b.log"] 
## 指定时区
# override_timezone = "Asia/Shanghai" 
## metrics是否带时间戳，注意，这里是"true"
# emit_metric_timestamp = "true" 

...
```
2. 在/path/to/prog1 目录下编写规则文件
```
gauge xxx_errors
/ERROR.*/ {
    xxx_errros++
}
```

3. 一个tab中执行 `categraf --test --inputs mtail`，用于测试 
4. 另一个tab中，"/path/to/a.log" 或者 "path/to/b.log" 追加一行 ERROR，看看categraf的输出
5. 测试通过后，启动categraf

### 输入
logs参数指定要处理的日志源, 支持模糊匹配, 支持多个log文件。

### 处理规则
`progs`指定具体的规则文件目录(或文件)


## 处理规则与语法

### 处理流程
```python 
for line in lines:
  for regex in regexes:
    if match:
      do something
```

### 语法

``` golang
exported variable 

pattern { 
  action statements
} 

def decorator { 
  pattern and action statements
}
```

#### 定义指标名称
前面也提过，指标仅支持 counter gauge histogram 三种类型。
一个🌰
```golang
counter lines
/INFO.*/ {
    lines++
}
```

注意，定义的名称只支持 C类型的命名方式(字母/数字/下划线)，**如果想使用"-" 要使用"as"导出别名**。例如，
```golang
counter lines_total as "line-count"
```
这样获取到的就是line-count这个指标名称了

#### 匹配与计算（pattern/action)

```golang
PATTERN {
ACTION
}
```

例子
```golang
/foo/ {
  ACTION1
}

variable > 0 {
  ACTION2
}

/foo/ && variable > 0 {
  ACTION3
}
```
支持RE2正则匹配
```golang
const PREFIX /^\w+\W+\d+ /

PREFIX {
  ACTION1
}

PREFIX + /foo/ {
  ACTION2
}
```

这样，ACTION1 是匹配以小写字符+大写字符+数字+空格的行，ACTION2 是匹配小写字符+大写字符+数字+空格+foo开头的行。

#### 关系运算符
+ `<` 小于 `<=` 小于等于
+ `>` 大于 `>=` 大于等于
+ `==` 相等 `!=` 不等
+ `=~` 匹配(模糊) `!~` 不匹配(模糊)
+ `||` 逻辑或 `&&` 逻辑与 `!` 逻辑非
 
#### 数学运算符
+ `|` 按位或
+ `&` 按位与
+ `^` 按位异或
+ `+ - * /` 四则运算
+ `<<` 按位左移
+ `>>` 按位右移
+ `**` 指数运算 
+ `=` 赋值
+ `++` 自增运算
+ `--` 自减运算
+ `+=` 加且赋值

#### 支持else与otherwise
```golang
/foo/ {
ACTION1
} else {
ACTION2
}
```
支持嵌套
```golang
/foo/ {
  /foo1/ {
     ACTION1
  }
  /foo2/ {
     ACTION2
  }
  otherwise {
     ACTION3
  }
}
```

支持命名与非命名提取

```golang
/(?P<operation>\S+) (\S+) \[\S+\] (\S+) \(\S*\) \S+ (?P<bytes>\d+)/ {
  bytes_total[$operation][$3] += $bytes
}
```
增加常量label 
```python
# test.mtail
# 定义常量label env
hidden text env
# 给label 赋值 这样定义是global范围;
# 局部添加，则在对应的condition中添加
env="production"
counter line_total by logfile,env
/^(?P<date>\w+\s+\d+\s+\d+:\d+:\d+)/ {
    line_total[getfilename()][env]++
}
```
获取到的metrics中会添加上`env=production`的label 如下：
```python
# metrics
line_total{env="production",logfile="/path/to/xxxx.log",prog="test.mtail"} 4 1661165941788
```

如果要给metrics增加变量label，必须要使用命名提取。例如
```python
# 日志内容
192.168.0.1 GET /foo
192.168.0.2 GET /bar
192.168.0.1 POST /bar
```

``` python
# test.mtail
counter my_http_requests_total by log_file, verb 
/^/ +
/(?P<host>[0-9A-Za-z\.:-]+) / +
/(?P<verb>[A-Z]+) / +
/(?P<URI>\S+).*/ +
/$/ {
    my_http_requests_total[getfilename()][$verb]++
}
```

```python
# metrics
my_http_requests_total{logfile="xxx.log",verb="GET",prog="test.mtail"} 4242
my_http_requests_total{logfile="xxx.log",verb="POST",prog="test.mtail"} 42
```

命名提取的变量可以在条件中使用
```golang
/(?P<x>\d+)/ && $x > 1 {
nonzero_positives++
}
```

#### 时间处理
不显示处理，则默认使用系统时间

默认emit_metric_timestamp="false" （注意是字符串）
```
http_latency_bucket{prog="histo.mtail",le="1"} 0
http_latency_bucket{prog="histo.mtail",le="2"} 0
http_latency_bucket{prog="histo.mtail",le="4"} 0
http_latency_bucket{prog="histo.mtail",le="8"} 0
http_latency_bucket{prog="histo.mtail",le="+Inf"} 0
http_latency_sum{prog="histo.mtail"} 0
http_latency_count{prog="histo.mtail"} 0
```

参数 emit_metric_timestamp="true" (注意是字符串)
```
http_latency_bucket{prog="histo.mtail",le="1"} 1 1661152917471
http_latency_bucket{prog="histo.mtail",le="2"} 2 1661152917471
http_latency_bucket{prog="histo.mtail",le="4"} 2 1661152917471
http_latency_bucket{prog="histo.mtail",le="8"} 2 1661152917471
http_latency_bucket{prog="histo.mtail",le="+Inf"} 2 1661152917471
http_latency_sum{prog="histo.mtail"} 3 1661152917471
http_latency_count{prog="histo.mtail"} 4 1661152917471
```

使用日志的时间
```
Aug 22 15:28:32 GET /api/v1/pods latency=2s code=200
Aug 22 15:28:32 GET /api/v1/pods latency=1s code=200
Aug 22 15:28:32 GET /api/v1/pods latency=0s code=200
```

```
histogram http_latency buckets 1, 2, 4, 8
/^(?P<date>\w+\s+\d+\s+\d+:\d+:\d+)/ {
        strptime($date, "Jan 02 15:04:05")
	/latency=(?P<latency>\d+)/ {
		http_latency=$latency
	}
}
```

日志提取的时间，一定要注意时区问题，有一个参数 `override_timezone` 可以控制时区选择，否则默认使用UTC转换。
比如我启动时指定 `override_timezone=Asia/Shanghai`, 这个时候日志提取的时间会当做东八区时间 转换为timestamp， 然后再从timestamp转换为各时区时间时 就没有问题了,如图。
![timestamp](https://cdn.jsdelivr.net/gh/flashcatcloud/categraf@main/inputs/mtail/timestamp.png)
如果不带 `override_timezone=Asia/Shanghai`, 则默认将`Aug 22 15:34:32` 当做UTC时间，转换为timestamp。 这样再转换为本地时间时，会多了8个小时, 如图。
![timestamp](https://cdn.jsdelivr.net/gh/flashcatcloud/categraf@main/inputs/mtail/timezone.png)


---

## 变更记录：日志行预处理功能（内存优化）

### 问题背景

在生产环境中，使用 mtail 插件监控大量 JSON 格式的应用日志时，categraf 进程出现持续性内存溢出（OOM），即使已在 mtail 程序中配置了 `limit 10000` 和 `del ... after 24h` 仍无法解决。

典型场景特征：
- 日志为大体积 JSON 行（单行 30KB~50KB+），但 mtail 程序仅需其中少数几个字段
- 日志文件按天滚动，glob 模式匹配到大量历史文件（200+）
- 日志产生频率高（每秒数百行）

### 根因分析

内存溢出由多个因素叠加导致：

**1. 指标基数爆炸**
- `limit 10000` 在源码中未在插入时强制执行（`GetDatum()` 中标注 `// TODO Check m.Limit`），仅在 GC 时检查
- `del after 24h` 的过期时间戳在每次指标更新时被刷新，活跃指标永不过期
- 高基数标签（如 `responseMsg`）导致指标时间序列笛卡尔积爆炸
- histogram 类型每个标签组合产生 9 条时间序列（buckets + sum + count）

**2. 大行日志的内存放大**
- 原始数据流：文件读取（128KB buffer）→ 按 `\n` 切行 → `string()` 转换 → channel 传递 → 正则匹配
- 每行 50KB 的 JSON 日志在 `string()` 转换时分配完整的 Go string 对象
- Go 正则 `FindStringSubmatch` 的捕获组引用原始大字符串，阻止 GC 回收
- 200 个文件 × 每秒 10 行 × 50KB = 约 100MB/s 的原始数据吞吐

**3. 文件资源累积**
- glob 模式匹配所有历史日志文件，每个文件占用一个 goroutine + 文件描述符
- 过期文件清理依赖 `staleTimer`（24h 超时），期间资源持续占用

### 修复方案

新增**日志行字节级预处理**功能，在日志行从文件读取后、转换为 Go string 之前，在原始 `[]byte` 上进行过滤和裁剪，从根本上避免大字符串的内存分配。

#### 核心思路

```
原始流程：文件 → 128KB buffer → 按\n切行 → string(50KB) → channel → 正则匹配
优化流程：文件 → 128KB buffer → 按\n切行 → LineFilter([]byte) → string(200B) → channel → 正则匹配
```

一行 50KB 的 JSON 日志，如果只需要 `hubsCode`、`methodName`、`responseCode` 等字段，经过字节级过滤后缩减为几百字节，`string()` 转换仅分配这几百字节，内存节省 99%+。

#### 配置方式

在 `mtail.toml` 的 instance 中添加：

```toml
[[instances]]
progs = "/path/to/prog"
logs = ["/path/to/*.log"]

# JSON 字段提取：仅保留指定字段，丢弃其余内容
# 适用于 JSON 格式的日志行，非 JSON 行不受影响原样传递
json_extract_fields = ["hubsCode", "methodName", "responseCode", "responseMsg", "status", "costTime", "partnerId"]

# 行长度限制：截断超过指定字节数的行
# 作为安全兜底，防止异常超长行消耗过多内存
max_line_length = 4096
```

#### 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `json_extract_fields` | `[]string` | 空（不启用） | 从 JSON 日志行中仅提取指定字段名，重组为精简 JSON。非 JSON 行原样传递。字段不存在时静默跳过，不报错。 |
| `max_line_length` | `int` | 0（不启用） | 截断超过此字节数的行。设为 0 或不配置则不截断。 |

两个参数可以单独使用，也可以组合使用。组合使用时先执行 JSON 字段提取，再执行长度截断。

#### 行为细节

- `json_extract_fields` 配置的字段在 JSON 中不存在 → 跳过该字段，不报错
- 所有配置字段都不存在 → 返回原始行原样传递
- 日志行不是 JSON 格式（不以 `{` 开头）→ 原样传递，不影响非 JSON 日志
- 对 socket/pipe 类型的日志源，通过 Runtime 级别的 `LinePreprocessor` 提供同等功能（string 级别过滤，作为后备）

### 变更文件清单

| 文件路径 | 变更类型 | 说明 |
|----------|----------|------|
| `inputs/mtail/internal/tailer/logstream/linefilter.go` | 新增 | 字节级 `LineFilter` 实现：`NewJSONBytesFieldExtractor`、`NewMaxLineLengthBytesFilter`、`ChainLineFilters` |
| `inputs/mtail/internal/tailer/logstream/reader.go` | 修改 | `LineReader` 增加 `filter` 字段和 `SetFilter()` 方法；`send()` 和 `Finish()` 在 `string()` 转换前应用 filter |
| `inputs/mtail/internal/tailer/logstream/filestream.go` | 修改 | `fileStream` 增加 `filter` 字段；`newFileStream()` 接受 `LineFilter` 参数并传递给 `LineReader` |
| `inputs/mtail/internal/tailer/logstream/logstream.go` | 修改 | `New()` 函数签名增加 `filter LineFilter` 参数，传递给 `newFileStream()` |
| `inputs/mtail/internal/tailer/tail.go` | 修改 | `Tailer` 增加 `filter` 字段；新增 `SetLineFilter()` Option；`TailPath()` 将 filter 传递给 `logstream.New()` |
| `inputs/mtail/internal/mtail/options.go` | 修改 | 新增 `SetLineFilter()` Server Option，将 `LineFilter` 传递给 Tailer |
| `inputs/mtail/internal/runtime/linepreprocessor.go` | 新增 | Runtime 级别的 `LinePreprocessor`（string 级别过滤，作为非文件流的后备方案） |
| `inputs/mtail/internal/runtime/runtime.go` | 修改 | 消费循环中集成 `linePreprocessor`，在分发给 VM 前对日志行进行预处理 |
| `inputs/mtail/internal/runtime/options.go` | 修改 | 新增 `SetLinePreprocessor()` Runtime Option |
| `inputs/mtail/mtail.go` | 修改 | `Instance` 结构体增加 `json_extract_fields` 和 `max_line_length` 配置项；`Init()` 中构建字节级 `LineFilter` 和 Runtime 级 `LinePreprocessor` 并注入 |

### 数据流架构

```
mtail.toml 配置
    │
    ▼
Instance.Init()
    ├── 构建 LineFilter (字节级) ──→ mtail.SetLineFilter Option
    │                                    │
    │                                    ▼
    │                              Tailer.filter
    │                                    │
    │                                    ▼
    │                          logstream.New(... filter)
    │                                    │
    │                                    ▼
    │                          newFileStream(... filter)
    │                                    │
    │                                    ▼
    │                          LineReader.SetFilter(filter)
    │                                    │
    │                                    ▼
    │                          send()/Finish() 中在 string() 前过滤 ← 核心优化点
    │
    └── 构建 LinePreprocessor (string级) ──→ Runtime.linePreprocessor
                                                │
                                                ▼
                                        消费循环中对 LogLine.Line 过滤（后备方案）
```
