# PoC verification — 2026-09-06

対象: Issue #1のread-only Gateway・UI。Echo Show実機での確認は含みません。

## 実施結果

| 検証 | 結果 |
| --- | --- |
| `go test ./...` / `go test -race ./...` | 成功。CLI JSON、Agent対応、状態優先順位、部分取得、fake CLIプロセス、HTTPエラー、同時取得・キャンセルを検証 |
| `go vet ./...` / `go build` | 成功 |
| `npm run build` | TypeScript検査・Vite production build成功 |
| `npm test` | Chromiumで8件成功。Goからproduction buildとdemo APIを配信 |
| 画面確認 | 合成データの960×480と960×360のスクリーンショットを確認。375×667の横はみ出しも自動検証 |
| 実Orca接続 | Orca 1.4.197、runtime ready。GatewayのHTTP APIから5 Terminal取得、partial=false / demo=false。Codex running/doneとunknown Terminalを分離 |

実Orcaのメッセージ・パス・handleは検証資料へ保存していません。実際のOrcaを停止・再起動したり、Terminalへ入力を送ったりしていません。

macOSのsandbox内ではChromiumの起動が拒否されたため、許可後にsandbox外でブラウザテストを実行しました。8件の成功はその実行結果です。

GitHub Actionsには同じ検証を構成しています。リモートCIの実行結果はPRのChecksで確認できます。

## 未確認・対象外

- Echo ShowのLAN接続、実機表示、タッチ、長時間維持、ホーム復帰、Wi-Fi復帰。
- Claude Codeの実セッション固有のJSONとNeeds Input / Failed判定。
- 本物のOrca再起動時のhandle変更。UIのhandle差し替えは合成データで検証済み。
- 操作API、認証、外部公開、SSE/WebSocket、OS改造。

再現手順はREADMEを参照してください。ブラウザテストのスクリーンショット・失敗時traceはローカルの `web/test-results/` に出力され、Gitには含めません。
