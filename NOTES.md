# NOTES

## 2026-08-08 — network.go 错误信封读错字段，401 被静默吞成空列表

### 目标
对生产环境跑 SDK 端到端实测，覆盖 marketplace 与已并入其中的 federation/network 能力，确认改名后新符号（`Network*`）与旧别名（`Federation*`）都可用。实测中发现并修掉一个真 bug。

### 症状
`NetworkPeers` 用一个无效占位凭证调用生产，返回 `peers=[]` 且 **无错误**。同样场景下 python SDK 正确抛出 401。一个应当失败的调用被静默放行成空结果，调用方无法区分"网络里没有 peer"和"你没认证"。

### 根因
服务端错误体的字段名是 `message`，而 `network.go` 里这两处的响应 struct 只声明了 `Error`：

- `NetworkPeers`（原 54 行）
- `ListAPIs`（原 331 行）

于是 `resp.Error` 恒为空串，守卫 `if !resp.Success && resp.Error != ""` 永不触发，函数带着零值列表正常返回。同文件另外 3 处守卫读的是 `message` 且只判 `!resp.Success`，写法是对的 —— 说明这是这两处的局部疏漏，不是全局设计问题。

### 关键决策及理由
- **响应 struct 同时声明 `Message` 和 `Error` 两个字段**，而不是把 `Error` 改名成 `Message`。服务端历史上两种字段都出现过，双声明对两种信封都成立，且不破坏任何已有调用方。
- **守卫简化为只判 `!resp.Success`**，去掉 `!= ""` 短路。`success=false` 本身就是失败的充分信号，错误文案缺失不应该让失败变成成功。这也与同文件其余 3 处对齐。
- **错误文案走新 helper `firstNonEmpty(Message, Error, "request failed")`**，保证任何信封组合下都有非空错误信息，不会出现 `error: ` 空消息。helper 定义追加在 `network.go` 尾部，循仓内 helper 就近放置的惯例，没有另起 utils 文件。
- **回归测试命名沿用仓内既有的 `...SurfacesSuccessFalse` 后缀**，插在已有的 `TestContractNetworkPeers` / `TestContractListAPIs` 附近，复用现成的 `newStub` 辅助。

### 被否方案
- **把 `Error` 字段改名为 `Message`**：否。会破坏任何已经读 `.Error` 的调用方，且服务端两种信封都可能出现，改名只是把漏洞换个方向。
- **在 HTTP 层统一拦截 `success=false`**：否。范围远超本次 bug，会牵动全部 5 处守卫和其他 client 的既有行为，属于该单独立项的重构，不该混在一次 e2e 修复里。

### 验证
- `go build ./...` / `go vet ./...` / `go test ./...` 全部 exit 0
- **变异验证三元组**（防止测试是恒真空壳）：
  - 把两处守卫临时回退成有 bug 的写法 → 两个新测试 FAIL，报 `expected error when success=false on a 200, got peers=[]`，精确复现原症状
  - 还原 → 两个测试 PASS，源文件与改动前**字节级一致**（备份已删）
  - 对照组 `TestContractListUserAPIsSurfacesSuccessFalse`（守卫本来就正确）→ 两轮都 PASS，证明变异只影响目标方法
- 生产 e2e：修复后 `NetworkPeers` 由"放行 peers=0"改为正确报错；9/9 项通过；真实数据落点 `resource_id=476 'Qr Code' 0.00575`

### 坑
- `go test` 的参数列表里**不要**塞 `2>&1`。用 `subprocess` 传 list 时它会被当成包名参数，结果是 `testing: warning: no tests to run`，看起来像正则没匹配上，实际是调用方式错了。重定向交给 `capture_output` 即可。
- 端到端探针 `e2e_network_probe/` 是一次性工具，验完即删，不入仓。

### 未验证项
- 只验证了 `NetworkPeers` 和 `ListAPIs` 这两处。另外 3 处守卫是读代码判定为正确，**没有**为它们逐个写 `success=false` 的回归测试。
- 服务端在什么条件下发 `error` 而非 `message`，没有追查；双字段声明是防御性覆盖，不代表已穷举服务端信封。
