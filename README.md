# Echo Show Agent Deck

Orcaで動くAIエージェントを、ブラウザから確認するread-onlyダッシュボード。
Go GatewayがOrca CLIの構造化JSONを正規化し、React UIへ返します。
Echo ShowではSilkを表示クライアントとして使います。PoCはMac/PCで検証し、Echo Showの実機確認は後続です。

## 起動

必要環境: Go 1.24以上、Node.js 22.12以上、npm。実接続には起動済みOrcaとCLIが必要です。

```sh
npm --prefix web ci
npm --prefix web run build
go run ./cmd/server
```

`http://127.0.0.1:8080` を開きます。停止はCtrl-C。

Orca不要のデモ:

```sh
go run ./cmd/server -demo
```

デモは合成fixtureを使い、画面にDEMOと表示します。接続失敗時に自動でデモへ切り替わることはありません。

設定:

```sh
go run ./cmd/server -orca /Applications/Orca.app/Contents/Resources/bin/orca -timeout 5s
go run ./cmd/server -addr 0.0.0.0:8080
```

`-addr` の初期値はloopbackです。LANでEcho Showから試すときだけ明示的に変更し、`http://MacのLAN-IP:8080` を使います。PoCには認証がなく、LAN bindすると到達可能な端末からセッション名・メッセージを読めます。信頼できるLANで使用してください。外部公開は対象外です。

配布ビルドは `go build -o bin/agent-deck ./cmd/server`。`bin/agent-deck` と `web/dist` を一緒に配置し、`-web /path/to/dist` で静的ファイルの位置を指定できます。

UI開発はGatewayを起動した上で `npm --prefix web run dev`。Viteが `/api` をlocalhost:8080へproxyします。

## APIと状態

`GET /api/sessions` は `{sessions: [...], meta: {partial, demo, fetchedAt}}` を返します。時刻はUnix milliseconds、欠損値はnullです。connected/writableも欠損時はnullです。結果は1秒共有、UIは5秒ごとに取得します。同時アクセス時のCLI実行は共有します。

Orca 1.4.197の実JSONで確認した `worktreeId` と `agents[].paneKey == terminal.tabId + ":" + terminal.leafId` を使って対応付けます。一意に対応しないAgentの情報は流用しません。種別は対応Agent → terminal.agentIdentity → unknownの順です。unknownはAPIに含み、UIでは切り替えで表示します。ephemeral Terminalは除外します。

| 表示 | 条件（上が優先） |
| --- | --- |
| Offline | Terminalが明示的にconnected=false |
| Interrupted | 対応Agentがinterrupted=true |
| Running | 対応Agentがstate=working |
| Done | 対応Agentがstate=done |
| Active workspace | WorktreeがworkspaceStatus=in-progress |
| Unknown | 上記の条件では判断できない |

Active workspaceはAgentが実行中という意味ではありません。Needs Input / Failedの推定やpreview解析は行いません。lastMessageは対応したAgentの最終発言のみ、lastActivityAtはTerminal出力時刻と対応Agent更新時刻の新しい方です。時刻降順・同時刻はID順、欠損は末尾です。

hostScopeの欠損・未収録ホスト・truncated・件数不一致・JOIN不成立はmeta.partialで表します。不完全な取得を「全セッションの終了」とは解釈しません。handleはruntime内限定で保存せず、画面は取得ごとに置き換えます。

エラー形式は `{error: {code: "..."}}`。CLI不在/既知の利用不可エラーは503 `orca_unavailable`、上流失敗は502 `upstream_error`、不正JSONは502 `invalid_response`、時間切れは504 `timeout`。未認識のCLIエラーは502です。CLIのstderrや詳細なエラーはHTTPへ露出しません。失敗時に以前の成功結果を正常応答として返しません。UIが前回結果を残す際は情報の古さを明示します。

APIのGET以外は405、未定義APIは404です。send、continue、stop、新規Agent起動、任意shell実行のAPIはありません。

## 検証

```sh
go test -race ./...
go vet ./...
npm --prefix web run build
cd web
npx playwright install chromium
npm test
```

Goテストは合成fixture、fake CLIプロセス、HTTP、キャッシュを検証します。ブラウザテストはGoからproduction buildとdemo APIを配信し、初期表示・更新・未知Terminal・エラー/復旧・HTMLのテキスト表示・960×480/960×360/375×667を確認します。Orcaの起動や人間の操作は不要で、GitHub Actionsでも同じ検証を実行します。

実接続の確認:

```sh
orca status --json
go run ./cmd/server
# 別のTerminalで
curl --fail http://127.0.0.1:8080/api/sessions
```

実JSONのprompt・ローカルパス・Terminal IDをfixtureや公開ログへ保存しないでください。

## 後続

実装計画は [Issue #1](https://github.com/taiseimiyaji/echo-show-tools/issues/1)。Echo Show実機でのLAN接続、表示/タッチ、長時間表示、ホーム復帰条件、Wi-Fi復帰は未検証です。Claude固有の実データ、Needs Inputの構造化条件、実際のOrca再起動時の挙動も後続です。これらをPC上のPoC完成の前提にはしません。

操作API、認証、SSE/WebSocket、外部公開、OS改造はこのPoCの範囲外です。
