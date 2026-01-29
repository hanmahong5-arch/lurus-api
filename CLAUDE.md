# Lurus API 项目开发指南 / Project Development Guide

## 文件读取规则 / File Reading Rules

### 必须阅读的文件 / Required Reading

1. **开始任何任务前必须阅读 / Read before starting any task:**
   - `doc/develop-guide.md` - 开发指南（如存在）/ Development guide (if exists)
   - `doc/plan.md` - 计划文档 / Planning document
   - `doc/process.md` - 开发进度 / Development progress

2. **涉及 API 开发时阅读 / Read when working on API:**
   - `router/` 目录 - 路由配置 / Route configurations
   - `controller/` 目录 - 控制器实现 / Controller implementations
   - `doc/meilisearch-integration.md` - 搜索集成文档 / Search integration docs

3. **涉及部署时阅读 / Read when deploying:**
   - `deploy/k8s/` - Kubernetes 配置 / K8s configurations
   - `docker-compose.yml` - Docker 配置 / Docker configuration
   - `DEPLOYMENT.md` - 部署指南 / Deployment guide
   - `重要信息.md` - 敏感配置（仅本地存在）/ Sensitive configs (local only)

4. **涉及前端开发时阅读 / Read when working on frontend:**
   - `web/src/` - 前端源码 / Frontend source code
   - `web/package.json` - 依赖配置 / Dependency configuration

---

## 敏感信息处理 / Sensitive Information

- **重要信息.md** 包含生产环境的敏感配置 / Contains production sensitive configs
- 此文件不在 Git 仓库中，仅存在于本地 / Not in Git, local only
- 部署或配置问题需要参考此文件 / Refer to this file for deployment/config issues

---

## API 文档 / API Documentation

- **在线文档 / Online Docs:** https://docs.lurus.cn/
- **API 入口 / API Entry:** https://api.lurus.cn/
- 访问 api.lurus.cn 后，点击页面上的"文档"按钮可跳转到 API 文档
- Access api.lurus.cn and click the "Docs" button to navigate to API documentation

---

## 技术栈 / Tech Stack

| 层级 / Layer | 技术 / Technology | 版本 / Version |
|--------------|-------------------|----------------|
| 后端 / Backend | Go | 1.25.1 |
| Web 框架 / Web Framework | Gin | latest |
| ORM | GORM | latest |
| 前端 / Frontend | React | 18 |
| 构建工具 / Build Tool | Vite | latest |
| UI 组件 / UI Components | Semi UI | latest |
| 搜索引擎 / Search Engine | Meilisearch | v1.10+ |
| 数据库 / Database | PostgreSQL (prod) / SQLite (dev) | - |
| 部署 / Deployment | K3s, Docker | - |

---

## 编码规范 / Coding Standards

1. **注释语言 / Comment Language:** 所有代码注释使用英文 / All code comments in English
2. **字符编码 / Character Encoding:** 所有字符编码使用 UTF-8 / Use UTF-8 for all files
3. **文档语言 / Documentation Language:** 使用中英双语 / Bilingual (Chinese & English)
4. **敏感信息 / Sensitive Info:** 不得在代码中硬编码敏感信息 / Never hardcode sensitive info
5. **格式化 / Formatting:** Go 代码遵循 `gofmt` 格式 / Go code follows `gofmt`

---

## 项目结构 / Project Structure

```
lurus-api/
├── cmd/server/              # Entry point (main.go)
├── internal/
│   ├── biz/                 # Business logic
│   │   ├── service/         # Service layer
│   │   └── relay/           # AI model relay logic
│   ├── data/
│   │   └── model/           # Data models (GORM)
│   ├── server/
│   │   ├── controller/      # API controllers
│   │   ├── middleware/       # HTTP middleware
│   │   └── router/          # Route definitions
│   └── pkg/                 # Internal shared utilities
│       ├── common/          # Common helpers
│       ├── constant/        # Constants
│       ├── dto/             # Data transfer objects
│       ├── logger/          # Logger setup
│       ├── search/          # Meilisearch integration
│       ├── setting/         # Settings (ratio, model, system, etc.)
│       └── types/           # Shared types
├── pkg/                     # Public packages (ionet)
├── web/                     # Frontend (React)
├── deploy/                  # Deployment configs
│   └── k8s/                 # Kubernetes manifests
├── doc/                     # Documentation
├── CLAUDE.md                # This file
├── README.md                # Project readme
└── 重要信息.md               # Sensitive info (local only, gitignored)
```

---

## 开发流程 / Development Workflow

1. **任务开始前 / Before starting a task:**
   - 阅读 `doc/plan.md` 了解项目规划 / Read plan.md for project planning
   - 阅读 `doc/process.md` 了解当前进度 / Read process.md for current progress

2. **开发过程中 / During development:**
   - 遵循编码规范 / Follow coding standards
   - 为代码添加英文注释 / Add English comments to code
   - 处理边缘情况 / Handle edge cases

3. **任务完成后 / After completing a task:**
   - 更新 `doc/process.md` 记录进度 / Update process.md with progress
   - 更新 `README.md`（如有新功能）/ Update README.md (for new features)
   - 维护 `.gitignore`（如有必要）/ Maintain .gitignore (if necessary)

---

## 常用命令 / Common Commands

### 后端开发 / Backend Development
```bash
# Build (silent mode)
cargo build -q 2>&1  # For Rust
go build -o lurus-api ./cmd/server  # For Go

# Run
./lurus-api

# Test
go test ./...
```

### 前端开发 / Frontend Development
```bash
cd web
**Always use `bun`, not `npm`.**
**始终使用 `bun`,而不是 `npm`。**

```sh
# 1. Make changes | 进行修改

# 2. Typecheck (fast) | 类型检查（快速）
bun run typecheck

# 3. Run tests | 运行测试
bun run test -- -t "test_name"     # Single suite | 单个测试套件
bun run test:file -- "glob"         # Specific files | 特定文件

# 4. Lint before committing | 提交前进行 Lint 检查
bun run lint:file -- "file1.ts"    # Specific files | 特定文件
bun run lint                        # All files | 所有文件

# 5. Before creating PR | 创建 PR 之前
bun run lint:claude && bun run test
```

### Docker
```bash
docker-compose up -d           # Start all services
docker-compose logs -f         # View logs
docker-compose down            # Stop services
```

---

## 注意事项 / Important Notes

1. **不得在任何文件中出现辅助开发的模型名称或 CLI 名称**
   Do not include AI model names or CLI names in any files

2. **测试需要隔离开发环境，建立 test-date-time 命名的文件夹**
   Tests should be isolated with test-date-time named folders

3. **界面交互设计之前需要进行 UX 分析**
   Conduct UX analysis before UI/UX design

4. **优先使用 Playwright 进行前端调试**
   Prefer Playwright for frontend debugging

5. **编译测试只做 debug 版本，除非有要求 release 版**
   Build debug versions for testing unless release is required
