/**
 * Cloudflared 插件 - 主应用逻辑
 */

// 平台映射表
const PLATFORM_MAP = {
    'darwin-amd64':  { file: 'cloudflared-darwin-amd64.tgz',   needsExtract: true },
    'darwin-arm64':  { file: 'cloudflared-darwin-arm64.tgz',   needsExtract: true },
    'linux-amd64':   { file: 'cloudflared-linux-amd64',        needsExtract: false },
    'linux-arm64':   { file: 'cloudflared-linux-arm64',        needsExtract: false },
    'linux-armv7':   { file: 'cloudflared-linux-arm',          needsExtract: false },
    'windows-amd64': { file: 'cloudflared-windows-amd64.exe',  needsExtract: false },
    'windows-arm64': { file: 'cloudflared-windows-amd64.exe',  needsExtract: false },
};

// 全局状态
let currentTab = 'home';
let pollTimer = null;
let serverPort = '58091';
let serverPlatform = 'linux-amd64';

// ============================================
// 初始化
// ============================================

document.addEventListener('DOMContentLoaded', async () => {
    // 初始化 Tab 切换
    document.querySelectorAll('.tab-item').forEach(tab => {
        tab.addEventListener('click', () => switchTab(tab.dataset.tab));
    });

    // 获取服务器端口和平台信息
    try {
        serverPort = await getServerPort();
        document.getElementById('server-port').textContent = serverPort;
    } catch (e) {
        console.error('获取端口失败:', e);
    }

    try {
        serverPlatform = await getServerPlatform();
        document.getElementById('detected-platform').textContent = serverPlatform;
    } catch (e) {
        console.error('获取平台信息失败:', e);
    }

    // 加载首页状态
    await refreshStatus();
});

// ============================================
// Tab 切换
// ============================================

function switchTab(tabName) {
    currentTab = tabName;

    // 更新 Tab 按钮状态
    document.querySelectorAll('.tab-item').forEach(tab => {
        tab.classList.toggle('active', tab.dataset.tab === tabName);
    });

    // 更新 Tab 页面显示
    document.querySelectorAll('.tab-page').forEach(page => {
        page.classList.toggle('active', page.id === 'tab-' + tabName);
    });

    // 切换到设置页时加载 release 信息
    if (tabName === 'settings') {
        loadReleaseInfo();
    }
}

// ============================================
// 首页功能
// ============================================

async function refreshStatus() {
    try {
        const resp = await apiGet('/api/status');
        if (resp && resp.data) {
            updateStatusUI(resp.data);
        }
    } catch (e) {
        console.error('获取状态失败:', e);
    }
}

function updateStatusUI(data) {
    const statusDot = document.getElementById('status-dot');
    const statusText = document.getElementById('status-text');
    const versionText = document.getElementById('installed-version');
    const startBtn = document.getElementById('btn-start');
    const stopBtn = document.getElementById('btn-stop');
    const tunnelCard = document.getElementById('tunnel-card');

    if (!data.installed) {
        statusDot.className = 'status-dot stopped';
        statusText.textContent = '未安装';
        versionText.textContent = '-';
        startBtn.disabled = true;
        stopBtn.classList.add('hidden');
        tunnelCard.classList.add('hidden');
        stopPolling();
        return;
    }

    versionText.textContent = data.version || '已安装';

    if (data.running) {
        statusDot.className = 'status-dot running';
        statusText.textContent = '运行中';
        startBtn.classList.add('hidden');
        stopBtn.classList.remove('hidden');
        tunnelCard.classList.remove('hidden');
        startPolling();
    } else {
        statusDot.className = 'status-dot stopped';
        statusText.textContent = '已停止';
        startBtn.classList.remove('hidden');
        startBtn.disabled = false;
        stopBtn.classList.add('hidden');
        tunnelCard.classList.add('hidden');
        stopPolling();
    }
}

// 启动隧道
async function startTunnel() {
    const btn = document.getElementById('btn-start');
    btn.disabled = true;
    btn.innerHTML = '<span class="material-symbols-outlined">hourglass_empty</span> 启动中...';

    try {
        const resp = await apiPost('/api/start', { port: serverPort });
        if (resp && resp.data && resp.data.message) {
            showSnackbar(resp.data.message);
        }
        // 延迟刷新状态
        setTimeout(refreshStatus, 1000);
    } catch (e) {
        showSnackbar('启动失败: ' + e.message);
        btn.disabled = false;
        btn.innerHTML = '<span class="material-symbols-outlined">play_arrow</span> 启动隧道';
    }
}

// 停止隧道
async function stopTunnel() {
    const btn = document.getElementById('btn-stop');
    btn.disabled = true;

    try {
        const resp = await apiPost('/api/stop', {});
        if (resp && resp.data && resp.data.message) {
            showSnackbar(resp.data.message);
        }
        stopPolling();
        setTimeout(refreshStatus, 500);
    } catch (e) {
        showSnackbar('停止失败: ' + e.message);
    } finally {
        btn.disabled = false;
    }
}

// ============================================
// 输出轮询
// ============================================

function startPolling() {
    if (pollTimer) return;
    pollOutput();
    pollTimer = setInterval(pollOutput, 2000);
}

function stopPolling() {
    if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
    }
}

async function pollOutput() {
    try {
        const resp = await apiGet('/api/output');
        if (!resp || !resp.data) return;
        const data = resp.data;

        // 合并 stdout 和 stderr
        const output = (data.stderr || '') + (data.stdout || '');
        const logEl = document.getElementById('log-output');
        if (logEl && output) {
            logEl.textContent = output;
            logEl.scrollTop = logEl.scrollHeight;
        }

        // 解析 trycloudflare.com 域名
        const urlMatch = output.match(/https?:\/\/[a-zA-Z0-9-]+\.trycloudflare\.com/);
        if (urlMatch) {
            const tunnelUrl = urlMatch[0];
            const urlEl = document.getElementById('tunnel-url');
            const linkEl = document.getElementById('tunnel-link');
            if (urlEl && linkEl) {
                linkEl.href = tunnelUrl;
                linkEl.textContent = tunnelUrl;
                urlEl.classList.remove('hidden');
            }
        }

        // 检查进程是否已停止
        if (data.running === false) {
            stopPolling();
            refreshStatus();
        }
    } catch (e) {
        console.error('轮询输出失败:', e);
    }
}

// 复制隧道 URL
function copyTunnelUrl() {
    const linkEl = document.getElementById('tunnel-link');
    if (linkEl && linkEl.textContent) {
        navigator.clipboard.writeText(linkEl.textContent).then(() => {
            showSnackbar('已复制到剪贴板');
        }).catch(() => {
            showSnackbar('复制失败');
        });
    }
}

// ============================================
// 设置页功能
// ============================================

async function loadReleaseInfo() {
    const versionEl = document.getElementById('latest-version');
    const downloadBtn = document.getElementById('btn-download');

    versionEl.textContent = '加载中...';

    try {
        const resp = await apiGet('/api/releases');
        if (resp && resp.data && resp.data.tag_name) {
            versionEl.textContent = resp.data.tag_name;
            downloadBtn.disabled = false;
        } else {
            versionEl.textContent = '获取失败';
        }
    } catch (e) {
        versionEl.textContent = '获取失败';
        console.error('获取 release 信息失败:', e);
    }
}

let downloadPollTimer = null;

async function downloadCloudflared() {
    const btn = document.getElementById('btn-download');
    const progressBar = document.getElementById('download-progress');
    if (!PLATFORM_MAP[serverPlatform]) {
        showSnackbar('不支持的平台: ' + serverPlatform);
        return;
    }

    btn.disabled = true;
    btn.innerHTML = '<span class="material-symbols-outlined">downloading</span> 启动下载...';
    progressBar.classList.remove('hidden');
    progressBar.querySelector('.progress-linear-fill').style.width = '0%';
    progressBar.classList.add('progress-indeterminate');

    try {
        const resp = await apiPost('/api/download', { platform: serverPlatform });

        if (resp && resp.data && resp.data.success) {
            showSnackbar('下载任务已启动');
            btn.innerHTML = '<span class="material-symbols-outlined">downloading</span> 下载中...';
            progressBar.classList.remove('progress-indeterminate');
            // 开始轮询下载进度
            startDownloadPolling();
        } else {
            progressBar.classList.remove('progress-indeterminate');
            showSnackbar('启动下载失败: ' + (resp && resp.data ? resp.data.message : '未知错误'));
            btn.disabled = false;
            btn.innerHTML = '<span class="material-symbols-outlined">download</span> 下载最新版本';
            progressBar.classList.add('hidden');
        }
    } catch (e) {
        progressBar.classList.remove('progress-indeterminate');
        showSnackbar('启动下载失败: ' + e.message);
        btn.disabled = false;
        btn.innerHTML = '<span class="material-symbols-outlined">download</span> 下载最新版本';
        progressBar.classList.add('hidden');
    }
}

function startDownloadPolling() {
    if (downloadPollTimer) return;
    pollDownloadStatus();
    downloadPollTimer = setInterval(pollDownloadStatus, 1000);
}

function stopDownloadPolling() {
    if (downloadPollTimer) {
        clearInterval(downloadPollTimer);
        downloadPollTimer = null;
    }
}

async function pollDownloadStatus() {
    const btn = document.getElementById('btn-download');
    const progressBar = document.getElementById('download-progress');
    const progressFill = progressBar.querySelector('.progress-linear-fill');

    try {
        const resp = await apiGet('/api/download/status');
        if (!resp || !resp.data) return;
        const data = resp.data;

        if (data.status === 'downloading') {
            // 更新进度条
            if (data.progress_percent > 0) {
                progressFill.style.width = data.progress_percent + '%';
            }
            // 更新按钮文字显示进度
            if (data.total_bytes > 0) {
                const downloadedMB = (data.downloaded_bytes / 1024 / 1024).toFixed(1);
                const totalMB = (data.total_bytes / 1024 / 1024).toFixed(1);
                btn.innerHTML = '<span class="material-symbols-outlined">downloading</span> ' + downloadedMB + ' / ' + totalMB + ' MB';
            }
        } else if (data.status === 'completed') {
            stopDownloadPolling();
            progressFill.style.width = '100%';
            showSnackbar('下载完成');
            btn.disabled = false;
            btn.innerHTML = '<span class="material-symbols-outlined">download</span> 下载最新版本';
            // 刷新首页状态
            refreshStatus();
            // 刷新设置页版本信息
            loadReleaseInfo();
            setTimeout(() => {
                progressBar.classList.add('hidden');
            }, 2000);
        } else if (data.status === 'failed') {
            stopDownloadPolling();
            showSnackbar('下载失败: ' + (data.error || '未知错误'));
            btn.disabled = false;
            btn.innerHTML = '<span class="material-symbols-outlined">download</span> 下载最新版本';
            progressBar.classList.add('hidden');
        } else if (data.status === 'not_found') {
            stopDownloadPolling();
            showSnackbar('下载任务不存在');
            btn.disabled = false;
            btn.innerHTML = '<span class="material-symbols-outlined">download</span> 下载最新版本';
            progressBar.classList.add('hidden');
        }
    } catch (e) {
        console.error('轮询下载状态失败:', e);
    }
}

// ============================================
// Snackbar 通知
// ============================================

function showSnackbar(message) {
    const snackbar = document.getElementById('snackbar');
    snackbar.textContent = message;
    snackbar.classList.add('show');
    setTimeout(() => {
        snackbar.classList.remove('show');
    }, 3000);
}
