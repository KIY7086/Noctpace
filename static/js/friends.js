// 全局变量
let addFriendModal = null;
let currentSearchResults = [];

// DOM 加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    addFriendModal = document.getElementById('addFriendModal');
    
    // 初始化加载
    loadFriendRequests();
    loadFriendList();
    
    // 绑定搜索事件
    const searchInput = document.getElementById('friendSearch');
    searchInput.addEventListener('input', debounce(function(e) {
        filterFriendList(e.target.value.toLowerCase());
    }, 300));
});

// 加载好友请求列表
function loadFriendRequests() {
    fetch('/api/friend-requests')
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            const requestsContainer = document.getElementById('friendRequests');
            // 确保 data 存在且是数组
            const requests = Array.isArray(data) ? data : [];
            
            if (requests.length === 0) {
                requestsContainer.innerHTML = '<div class="empty-state">暂无好友请求</div>';
                return;
            }
            
            requestsContainer.innerHTML = requests.map(request => `
                <div class="friend-item request-item" data-request-id="${request.id}">
                    <div class="friend-avatar">
                        <i class="fas fa-user"></i>
                    </div>
                    <div class="friend-info">
                        <div class="friend-name">${request.username}</div>
                        <div class="friend-meta">
                            ${formatTime(request.created_at)}
                        </div>
                    </div>
                    <div class="friend-actions">
                        <button class="btn btn-primary" onclick="handleFriendRequest(${request.id}, 'accept')">
                            <i class="fas fa-check"></i> 接受
                        </button>
                        <button class="btn btn-secondary" onclick="handleFriendRequest(${request.id}, 'reject')">
                            <i class="fas fa-times"></i> 拒绝
                        </button>
                    </div>
                </div>
            `).join('');
        })
        .catch(error => {
            console.error('加载好友请求失败:', error);
            const requestsContainer = document.getElementById('friendRequests');
            requestsContainer.innerHTML = '<div class="empty-state">加载失败，请稍后重试</div>';
        });
}

// 加载好友列表
function loadFriendList() {
    fetch('/api/friends')
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            const friendListContainer = document.getElementById('friendList');
            const friends = Array.isArray(data) ? data : [];
            
            if (friends.length === 0) {
                friendListContainer.innerHTML = '<div class="empty-state">暂无好友，快去添加吧！</div>';
                return;
            }
            
            friendListContainer.innerHTML = friends.map(friend => {
                // 处理头像显示
                const avatarHtml = friend.avatar ? 
                    `<img src="${friend.avatar}" alt="${friend.username}">` : 
                    `<i class="fas fa-user"></i>`;

                return `
                    <div class="friend-item" data-friend-id="${friend.id}">
                        <div class="friend-avatar">
                            ${avatarHtml}
                        </div>
                        <div class="friend-info">
                            <div class="friend-name">${friend.username}</div>
                            <div class="friend-meta">
                                成为好友于 ${formatTime(friend.created_at)}
                            </div>
                        </div>
                        <div class="friend-actions">
                            <button class="btn btn-primary" onclick="startPrivateChat(${friend.id}, '${friend.username}')">
                                <i class="fas fa-comment"></i> 发消息
                            </button>
                        </div>
                    </div>
                `;
            }).join('');
        })
        .catch(error => {
            console.error('加载好友列表失败:', error);
            const friendListContainer = document.getElementById('friendList');
            friendListContainer.innerHTML = '<div class="empty-state">加载失败，请稍后重试</div>';
        });
}

// 处理好友请求
function handleFriendRequest(requestId, action) {
    if (!requestId || !action) return;
    
    fetch(`/api/friend-request/${action}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `request_id=${requestId}`
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        return response.json();
    })
    .then(data => {
        alert('好友请求已处理');
        // 重新加载好友请求和好友列表
        loadFriendRequests();
        loadFriendList();
    })
    .catch(error => {
        console.error('处理好友请求失败:', error);
        alert('处理好友请求失败，请重试');
    });
}

// 搜索用户
function searchUser() {
    const username = document.getElementById('usernameSearch').value.trim();
    if (!username) {
        alert('请输入用户名');
        return;
    }

    const searchResult = document.getElementById('searchResult');
    searchResult.innerHTML = '<div class="empty-state">搜索中...</div>';

    fetch(`/api/search-users?username=${encodeURIComponent(username)}`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            // 确保 data 是数组
            const users = Array.isArray(data) ? data : [];
            
            if (users.length === 0) {
                searchResult.innerHTML = '<div class="empty-state">未找到用户</div>';
                return;
            }

            searchResult.innerHTML = users.map(user => `
                <div class="friend-item">
                    <div class="friend-avatar">
                        <i class="fas fa-user"></i>
                    </div>
                    <div class="friend-info">
                        <div class="friend-name">${user.username}</div>
                    </div>
                    <div class="friend-actions">
                        <button class="btn btn-primary" onclick="sendFriendRequest(${user.id})">
                            <i class="fas fa-user-plus"></i> 添加好友
                        </button>
                    </div>
                </div>
            `).join('');
        })
        .catch(error => {
            console.error('搜索用户失败:', error);
            searchResult.innerHTML = '<div class="empty-state">搜索失败，请重试</div>';
        });
}

// 发送好友请求
function sendFriendRequest(userId) {
    if (!userId) return;

    fetch('/api/friend-request', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `target_user_id=${userId}`
    })
    .then(response => {
        if (!response.ok) {
            if (response.status === 409) {
                throw new Error('已经发送过好友请求或已经是好友');
            }
            throw new Error('Network response was not ok');
        }
        return response.json();
    })
    .then(data => {
        alert('好友请求已发送');
        hideAddFriendModal();
    })
    .catch(error => {
        console.error('发送好友请求失败:', error);
        alert(error.message || '发送好友请求失败，请重试');
    });
}

// 显示添加好友模态框
function showAddFriendModal() {
    const modal = document.getElementById('addFriendModal');
    const searchResult = document.getElementById('searchResult');
    const searchInput = document.getElementById('usernameSearch');
    
    modal.style.display = 'flex';
    searchResult.innerHTML = '<div class="empty-state">搜索用户开始添加好友</div>';
    searchInput.value = '';
}

// 隐藏添加好友模态框
function hideAddFriendModal() {
    const modal = document.getElementById('addFriendModal');
    modal.style.display = 'none';
}

// 工具函数
function formatTime(timestamp) {
    const date = new Date(timestamp);
    const now = new Date();
    const options = {
        month: '2-digit',
        day: '2-digit'
    };
    
    // 如果不是当前年份，添加年份显示
    if (date.getFullYear() !== now.getFullYear()) {
        options.year = 'numeric';
    }
    
    return date.toLocaleDateString('zh-CN', options).replace(/\//g, '-');
}

function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

function filterFriendList(searchText) {
    const friendItems = document.querySelectorAll('#friendList .friend-item');
    friendItems.forEach(item => {
        const friendName = item.querySelector('.friend-name').textContent.toLowerCase();
        if (friendName.includes(searchText)) {
            item.style.display = 'flex';
        } else {
            item.style.display = 'none';
        }
    });
}

// 点击空白处关闭模态框
window.onclick = function(event) {
    if (event.target === addFriendModal) {
        hideAddFriendModal();
    }
};

// 添加新的开始私聊函数
function startPrivateChat(userId, username) {
    // 保存聊天状态到 LocalStorage
    const chatState = {
        roomId: null, // 这个值会在主页通过 API 获取
        targetUser: username,
        isPublicRoom: false
    };
    localStorage.setItem('chatState', JSON.stringify(chatState));
    
    // 跳转到主页
    window.location.href = '/';
}