# 邀请功能网页测试方案

## 目标

后续使用 Playwright 从浏览器视角验证邀请功能 UI 和 API 串联是否正确。测试重点不是视觉重设计，而是确认不同用户角色看到的功能、按钮、统计数据和权限限制符合预期。

## 测试环境

- 使用 Docker dev 环境启动后端、数据库、Redis 和前端页面。
- 使用测试数据库种子数据创建三类用户：
  - 普通用户 N：没有邀请功能。
  - 可开启邀请功能的用户 A：管理员分配过管理员分配的邀请码，可邀请用户，可给直属 B 开启邀请功能。
  - 已开启邀请功能的用户 B：由 A 开启邀请功能，可邀请 C，但不能继续给 C 开启邀请功能。
  - 被邀请用户 C：由 B 邀请产生，有充值和消费数据。
- Playwright 通过真实浏览器打开充值页，使用测试 token 或登录流程进入对应用户态。

## 建议种子数据

### 用户

- A：`agent_a`
  - `invite_agent_level = 1`
  - `can_grant_invitation = true`
  - 拥有一个管理员分配的邀请码。
- B：`agent_b`
  - 由 A 的邀请码注册。
  - A 可给 B 开启邀请功能。
  - 开启后 B 拥有一个自动生成的邀请码。
- C：`agent_c`
  - 由 B 的自动生成的邀请码注册。
  - 有成功充值、消费额度和消费日志。
- N：`normal_user`
  - 无邀请功能。

### 统计

- B 本人：
  - 成功充值一笔。
  - 有 `used_quota`。
- C：
  - 成功充值一笔。
  - 有 `used_quota`。
  - 有一条 `LogTypeConsume` 消费日志。
- 再插入一笔 pending 充值，用于确认页面统计不展示为成功充值。

## 测试用例

### 1. 普通用户不显示邀请功能扩展模块

步骤：

1. 使用普通用户 N 打开充值页。
2. 定位 `邀请统计` 卡片。
3. 检查基础邀请统计仍存在。
4. 检查页面不显示 `邀请趋势`。
5. 检查页面不显示 `被邀请人邀请统计`。
6. 检查最近被邀请人列表没有 `开启邀请功能` 按钮。

预期：

- 普通用户不会看到邀请功能扩展入口。
- 页面没有空图表或无意义的二级统计表。

### 2. 可开启邀请功能的用户 A 可以看到并开启 B 的邀请功能

步骤：

1. 使用 A 打开充值页。
2. 检查概览卡片显示 `邀请功能已开启`。
3. 检查显示 `可给被邀请人开启邀请码`。
4. 检查 `邀请趋势` 卡片存在。
5. 在 `最近被邀请人` 中找到 B。
6. 点击 B 行内 `开启邀请功能`。
7. 在确认框点击确认。
8. 等待接口完成和页面刷新。

预期：

- B 的状态变成 `邀请功能已开启`。
- `被邀请人邀请统计` 表出现 B 行。
- B 行显示邀请码。
- B 本人充值/消费显示正确。

### 3. 可开启邀请功能的用户 A 查看 B 邀请用户统计

步骤：

1. 使用已开启 B 的 A 打开充值页。
2. 找到 `被邀请人邀请统计` 表。
3. 定位 B 的行。
4. 检查 `其邀请用户充值`、`其邀请用户消费` 数值。
5. 切换 `邀请趋势` 为 `按月`。
6. 等待图表刷新。

预期：

- `其邀请用户充值` 包含 C 的成功充值。
- `其邀请用户消费` 包含 C 的 `used_quota`。
- 图表切换月维度后页面不报错，横轴显示月维度标签。

### 4. 已开启邀请功能的用户 B 只能看自己的直属邀请统计

步骤：

1. 使用 B 打开充值页。
2. 检查概览卡片显示 `邀请功能已开启`。
3. 检查不显示 `可给被邀请人开启邀请码`。
4. 检查显示 `邀请趋势`。
5. 检查不显示 `被邀请人邀请统计`。
6. 检查 C 出现在最近被邀请人列表。
7. 检查 C 行没有 `开启邀请功能` 按钮。

预期：

- B 可以看到自己邀请 C 的充值/消费趋势。
- B 不能看到 A 的被邀请人邀请统计表。
- B 不能给 C 开启邀请功能。

### 5. 已开启邀请功能的用户 B 接口拒绝继续开通

步骤：

1. 使用 B 打开充值页。
2. 直接通过页面行为或测试辅助调用 `POST /api/user/self/invitees/:c_id/enable_invitation`。
3. 捕获响应或页面错误提示。

预期：

- 接口返回 `success: false`。
- 错误文案为无权限相关提示。
- 页面状态不变化，C 不会变成已开启。

### 6. 邀请人弹窗状态一致

步骤：

1. 使用 A 打开充值页。
2. 点击 `最近被邀请人` 的 `查看全部`。
3. 在弹窗表格中找到 B。
4. 检查 `邀请功能` 列显示 `已开启`。
5. 使用未开启下级邀请的测试用户时，检查该列显示 `开启` 按钮。

预期：

- 预览列表和弹窗表格状态一致。
- 开启后弹窗内状态也刷新。

## Playwright 实现建议

### 登录方式

优先使用测试辅助登录，避免 UI 登录流程影响用例稳定性：

1. 通过 API 获取或写入测试用户 token。
2. 在 Playwright 中写入 localStorage/cookie。
3. 打开充值页。

如果项目没有稳定的测试登录辅助，则先用 API 登录，再保存 storage state：

```ts
await page.goto('/login');
await page.getByPlaceholder('用户名').fill('agent_a');
await page.getByPlaceholder('密码').fill('password123');
await page.getByRole('button', { name: '登录' }).click();
await page.context().storageState({ path: 'playwright/.auth/agent-a.json' });
```

### 选择器策略

- 优先使用可见文本：`邀请趋势`、`被邀请人邀请统计`、`开启邀请功能`。
- 表格行用用户名定位：`agent_b`、`agent_c`。
- 后续如果文本翻译造成不稳定，可以给关键区域加 `data-testid`：
  - `invite-agent-trend`
  - `invite-second-level-table`
  - `invite-enable-button-{userId}`
  - `invite-agent-level-tag`

### 网络断言

关键操作可监听接口：

```ts
const responsePromise = page.waitForResponse((response) =>
  response.url().includes('/api/user/self/invitees/') &&
  response.url().includes('/enable_invitation') &&
  response.request().method() === 'POST',
);
await page.getByRole('button', { name: '开启邀请功能' }).click();
await page.getByRole('button', { name: '确定' }).click();
const response = await responsePromise;
expect(response.ok()).toBeTruthy();
```

### 图表断言

VChart canvas 不适合断言具体像素作为主校验。建议：

- 主要断言接口响应数据。
- 断言 `邀请趋势` 区域存在。
- 断言日/月切换后接口重新请求。
- 必要时检查 canvas 非空或截图作为辅助。

## 回归命令

建议后续新增脚本：

```bash
cd web
bunx playwright test tests/invite-agent.spec.ts
```

如果 Playwright 在仓库根目录统一管理，则使用根目录脚本：

```bash
bunx playwright test web/tests/invite-agent.spec.ts
```

## 验收标准

- 普通用户无邀请功能扩展模块。
- 可开启邀请功能的用户 A 能开启 B，并看到 B 本人和 B 邀请用户统计。
- 已开启邀请功能的用户 B 能邀请 C，但不能继续开启 C 的邀请能力。
- 日/月切换能刷新趋势。
- 弹窗列表和预览列表状态一致。
- 页面无前端运行时错误，移动端宽度下无明显文字重叠。
