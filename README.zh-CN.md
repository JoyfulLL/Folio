# Folio

|                      |                         |
| -------------------- | ----------------------- |
| [English](README.md) | [简体中文](README.zh-CN.md) |

一款轻量、可自托管的个人记账应用。Folio 记录日常交易，支持多种数据源导入，并提供搜索、筛选、图表与自定义查询来可视化收支。它可以轻松运行在家用服务器、NAS 或树莓派上，提供桌面端与移动端界面并支持 PWA。

## 主要特点

Folio 是基于 [ezBookkeeping](https://github.com/mayswind/ezbookkeeping) 的独立定制版本，fork 点为提交 `a92a1081`（2026-11-10）。以下是新增的主要特点，通用功能见下文。

### 信用卡

- **共享额度池** —— 一个信用卡账户作为共享额度容器，其下的子账户是各张实体卡。池内所有卡的已用额度汇总后计算剩余可用额度，并按各币种的池额度推导汇率完成跨币种换算。「实体卡」开关用于标记该账户是真实卡片还是仅作额度容器；本卡币种银行额度仅作记录参考，不参与额度计算。隐藏余额模式同样生效。
- **包含账单日当天交易** —— 可决定账单日当天发生的交易计入本期账单还是下期账单。该选择会同时作用于账单周期边界、日期范围筛选与统计，桌面端和移动端一致。
- **卡片资料** —— 发卡银行名称、卡片名称、卡号后四位、持卡人姓名、有效期月份／年份，以及单独上传的卡面图片（独立文件存储，并提供清理未使用图片的接口）。附加字段收在「更多卡片信息」折叠区。
- **还款日规则** —— 支持「账单日后 N 天」或「每月固定日期」两种模式。
- **信用卡邮件通知** —— 账单日提醒、账单邮件（包含刚结束账单周期的全部交易）、还款日提醒（可设置提前天数）。每位用户可指定所在时区的通知时间与 15 分钟至 12 小时的检查间隔；每 15 分钟的定时任务负责计划投递并补发遗漏的通知。

### 自动化

- **每日记账日报推送飞书** —— 通过飞书自定义机器人 Webhook 把每日收支汇总推送到群聊。逐用户设置包含 Webhook、签名密钥（保存前用服务器密钥加密）、自定义关键字、发送时间与「发送测试消息」。每 15 分钟的定时任务在设定本地时间或其之后投递（最多延迟 15 分钟），并记录投递日志。

### 界面

- **账户图标统一徽章** —— 账户卡片统一使用 40×40 圆形徽章，24px 图标 `contain` 等比居中，配主题色令牌；移除了覆盖整张卡片的大尺寸半透明水印。明暗主题共用同一套设计令牌。
- **洞察探索按钮显隐偏好** —— 可显示或隐藏桌面端导航栏的 Insights Explorer 入口（上游始终显示）。
- **Folio 品牌** —— 应用名、PWA 图标与导出文件名前缀统一为 Folio；「关于」页面标注「基于 ezBookkeeping 构建」。

### 构建与部署

- 提供拉取式原地升级脚本 `deploy/ez-update.sh`、systemd 单元与 MySQL 初始化 SQL。
- CI 打包工作流；发布资源命名为 `folio-<版本>-<架构>.tar.gz`，使用不带 `v` 前缀的纯 semver 标签，避免与上游 `v*.*.*` 冲突。
- 构建脚本、Dockerfile 与 PWA 工具链针对改名后的发布产物做了相应调整。

### 上游基线

本 fork 固定在上游 a92a1081（2026-11-10）。上游之后的改动 —— MT940 的 ING :86: 结构化字段解析、QIF 带空格日期与 Beancount 导入修复、MCP 工具注解、下拉菜单／文本框样式调整及翻译更新 —— 尚未合并。这是版本差距，不是有意移除功能。

## 功能

- **自托管与隐私** —— 数据留在自己的服务器上
- **轻量** —— 支持 SQLite、MySQL、PostgreSQL，资源占用小
- **跨平台** —— Windows、macOS、Linux、FreeBSD；x86、amd64 与 ARM
- **桌面端 + 移动端** —— PWA 安装、深色模式、可自定义总览布局
- **记账** —— 两级账户与分类、图片附件、地图、计划交易、高级筛选
- **导入导出** —— CSV、Excel、OFX、QFX、QIF、IIF、Camt.052/053、MT940、GnuCash、Firefly III、Beancount 等；自定义列映射、规则与脚本
- **本地化** —— 24 种语言、多币种、多种汇率来源、多时区、自定义日期与数字格式
- **安全** —— 两步验证、OIDC 外部登录、登录限速、应用锁（PIN / WebAuthn）
- **自动化** —— 每日记账日报、信用卡账单与还款提醒、飞书 / SMTP 通知、MCP 与 API 工具

## 从源码构建

需要 Go、GCC（SQLite 用）、Node.js 和 npm。

```sh
# 前端 + 后端构建
./build.sh all

# 发布包（生成 folio-<版本>-<架构>.tar.gz）
./build.sh package --release -o folio.tar.gz

# Docker 镜像
./build.sh docker
```

Windows 下对应 `build.bat` 和 `build.ps1`。

构建成功后，前端产物写入 `dist/`，后端可执行文件为 `ezbookkeeping`（Windows 下为 `ezbookkeeping.exe`）。可执行文件名与配置文件名保持不变，方便已有部署和升级脚本继续使用。

## 运行

```sh
./ezbookkeeping server run
```

默认监听 8080 端口：`http://{你的主机地址}:8080/`。

配置文件位于 `conf/ezbookkeeping.ini`。本地开发时复制为 `conf/ezbookkeeping.local.ini` 后运行：

```sh
go run ezbookkeeping.go server run --conf-path=conf/ezbookkeeping.local.ini
```

所有配置项都可以用 `EBK_<节名>_<键名>` 环境变量覆盖。

### Docker

```sh
docker run -p8080:8080 -v /path/to/data:/ezbookkeeping/data folio:local
```

为 `data/`、`log/`、`storage/` 挂载持久化卷，保证容器重建后数据不丢。

## 部署辅助工具

`deploy/ez-update.sh` 可对 systemd 安装执行拉取式原地升级。它只替换后端二进制、`public/`、`templates/` 和 `LICENSE`，不会覆盖真实配置、数据库或日志。目录约定见 `deploy/ez-update.env.example` 和 `deploy/ezbookkeeping.service`。

发布标签使用不带 `v` 前缀的纯 semver（`1.0.3`），避免与上游 `v*.*.*` 发布流程冲突。标签同时作为二进制与网页界面显示的版本号，发布资源命名为 `folio-<标签>-linux-<架构>.tar.gz`。

## 许可证

MIT。本项目是基于 MIT 许可的 ezBookkeeping 的独立定制版本；原始版权声明与第三方许可文本保留在 [LICENSE](LICENSE) 和 `third-party-dependencies.json` 中，并同时展示在应用的「关于」对话框里。
