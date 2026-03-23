# inputs

每个采集插件就是一个目录，大家可以点击各个目录进去查看，每个插件的使用方式，都提供了 README 和默认配置，一目了然。如果想贡献插件，可以拷贝 tpl 目录的代码，基于 tpl 做改动。

## 插件变更说明

- **mtail**: 新增日志行字节级预处理功能（`json_extract_fields`、`max_line_length`），用于解决大体积 JSON 日志场景下的内存溢出问题。详见 [mtail/Readme.md](mtail/Readme.md) 中「变更记录：日志行预处理功能（内存优化）」章节。