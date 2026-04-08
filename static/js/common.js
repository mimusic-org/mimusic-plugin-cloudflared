/**
 * Cloudflared 插件 - 公共 API 工具模块
 */

const API_BASE = '/api/v1/plugin/cloudflared';
const CONFIGS_API = '/api/v1/configs';

/**
 * 从 localStorage 获取认证 Token
 */
function getAuthToken() {
    try {
        const authData = localStorage.getItem('mimusic-auth');
        if (authData) {
            const auth = JSON.parse(authData);
            return auth.accessToken || '';
        }
    } catch (error) {
        console.error('获取 Token 失败:', error);
    }
    return '';
}

/**
 * 构建请求头（含可选的 Authorization）
 */
function buildHeaders() {
    const headers = { 'Content-Type': 'application/json' };
    const token = getAuthToken();
    if (token) {
        headers['Authorization'] = 'Bearer ' + token;
    }
    return headers;
}

/**
 * 发送 GET 请求并返回 JSON
 */
function apiGet(path) {
    return fetch(API_BASE + path, {
        method: 'GET',
        headers: buildHeaders()
    }).then(response => response.json());
}

/**
 * 发送 POST 请求并返回 JSON
 */
function apiPost(path, body) {
    return fetch(API_BASE + path, {
        method: 'POST',
        headers: buildHeaders(),
        body: JSON.stringify(body)
    }).then(response => response.json());
}

/**
 * 获取服务器端口号（从 configs API）
 */
function getServerPort() {
    return fetch(CONFIGS_API + '/server_port', {
        method: 'GET',
        headers: buildHeaders()
    }).then(response => response.json())
    .then(data => {
        if (data && data.value) {
            return data.value;
        }
        return '58091'; // 默认端口
    })
    .catch(() => '58091');
}

/**
 * 获取服务器平台信息（从 configs API）
 * 返回格式如：darwin-arm64, linux-amd64, windows-amd64
 */
function getServerPlatform() {
    return fetch(CONFIGS_API + '/server_platform', {
        method: 'GET',
        headers: buildHeaders()
    }).then(response => response.json())
    .then(data => {
        if (data && data.value) {
            return data.value;
        }
        return 'linux-amd64'; // 默认平台
    })
    .catch(() => 'linux-amd64');
}
