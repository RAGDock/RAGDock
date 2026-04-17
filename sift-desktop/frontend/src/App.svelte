<script>
    import { EventsOn } from '../wailsjs/runtime/runtime';
    import { SelectAndIndexFolder, SearchAndAsk } from '../wailsjs/go/main/App';
    import { fade, fly } from 'svelte/transition';

    let query = "";
    let lastQuery = "";
    let answer = "";
    let path = "";
    let status = "就绪";
    let syncMessage = "";
    let isSearching = false;

    EventsOn("file_synced", (msg) => {
        syncMessage = msg;
        setTimeout(() => { syncMessage = ""; }, 3000);
    });

    async function handleSearch() {
        if (!query) return;

        lastQuery = query; // ✅ 在开始搜索前，记录当前问题
        const currentQuery = query;
        query = ""; // ✅ 清空输入框，提升交互感

        answer = "";
        isSearching = true;
        status = "🔍 正在翻阅本地文档...";

        setTimeout(() => { if(isSearching) status = "🧠 Sift 正在思考中..."; }, 800);

        try {
            const res = await SearchAndAsk(currentQuery);
            status = "✍️ Sift 回答中...";
            answer = res;
            status = "✅ Sift 已回答完";
        } catch (e) {
            status = "❌ 出错了";
            answer = "错误原因: " + e;
        } finally {
            isSearching = false;
        }
    }
</script>

<main class="sift-app">
    {#if syncMessage}
        <div class="toast-notification" in:fly={{ y: -20, duration: 400 }} out:fade>
            {syncMessage}
        </div>
    {/if}

    <div class="content-wrapper">
        <header class="navbar">
            <div class="path-badge" on:click={() => SelectAndIndexFolder().then(p => path = p)}>
                <span class="dot"></span> {path || "关联知识库"}
            </div>
            <div class="status-tag" class:active={isSearching}>{status}</div>
        </header>

        <section class="display-container">
            {#if lastQuery}
                <div class="question-box" in:fade={{ duration: 400 }}>
                    <div class="meta">YOUR QUESTION</div>
                    <p>{lastQuery}</p>
                </div>
            {/if}

            {#if answer}
                <div class="answer-box" in:fade={{ duration: 600 }}>
                    <div class="meta">SIFT RESPONSE</div>
                    <p>{answer}</p>
                </div>
            {:else if isSearching}
                <div class="loading-state" in:fade>
                    <div class="typing-loader"></div>
                </div>
            {:else if !lastQuery}
                <div class="empty-state">
                    <h2>Sift</h2>
                    <p>你的离线隐私知识库已就绪</p>
                </div>
            {/if}
        </section>

        <footer class="input-section">
            <div class="input-pill">
                <input
                        bind:value={query}
                        placeholder="向你的本地文档提问..."
                        on:keypress={e => e.key === 'Enter' && handleSearch()}
                />
                <button class="action-btn" on:click={handleSearch} disabled={isSearching}>
                    {isSearching ? "●" : "→"}
                </button>
            </div>
        </footer>
    </div>
</main>

<style>
    /* 保持原有的黑白极简风格，增加对话框样式 */
    :global(body) { background: #ffffff; color: #121212; font-family: "Inter", system-ui, sans-serif; }

    .sift-app { height: 100vh; display: flex; flex-direction: column; align-items: center; }
    .content-wrapper { width: 100%; max-width: 720px; height: 100%; display: flex; flex-direction: column; padding: 20px; }

    .navbar { display: flex; justify-content: space-between; padding: 20px 0; border-bottom: 1px solid #f0f0f0; }
    .path-badge { cursor: pointer; font-size: 12px; font-weight: 500; display: flex; align-items: center; gap: 8px; }
    .dot { width: 8px; height: 8px; background: #00c853; border-radius: 50%; }
    .status-tag { font-size: 12px; color: #888; transition: color 0.3s; }

    .display-container { flex: 1; padding: 40px 0; overflow-y: auto; }

    /* 提问框样式 */
    .question-box {
        margin-bottom: 40px;
        padding: 0 10px;
        border-left: 2px solid #121212; /* 区分提问与回答 */
    }
    .question-box p { font-size: 20px; font-weight: 500; line-height: 1.4; margin: 0; }

    /* 回答框样式 */
    .answer-box { background: #fafafa; padding: 30px; border-radius: 20px; border: 1px solid #eee; }
    .meta { font-size: 10px; font-weight: 800; color: #bbb; letter-spacing: 1px; margin-bottom: 15px; }
    .answer-box p { line-height: 1.8; margin: 0; font-size: 16px; white-space: pre-wrap; }

    .input-pill {
        background: #fff; border: 1px solid #e0e0e0; border-radius: 16px;
        display: flex; padding: 8px; box-shadow: 0 4px 20px rgba(0,0,0,0.03);
    }
    input { flex: 1; border: none; padding: 12px 20px; outline: none; font-size: 16px; }
    .action-btn { background: #121212; color: #fff; border: none; width: 44px; height: 44px; border-radius: 12px; cursor: pointer; }

    .empty-state { text-align: center; margin-top: 20vh; }
    .empty-state h2 { font-size: 48px; font-weight: 200; margin: 0; color: #121212; }
</style>