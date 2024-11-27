let currentRoomId = null;
let currentTargetUser = null;
let currentTargetAvatar = null;

// 保存聊天状态到 LocalStorage
function saveChatState() {
    const chatState = {
        roomId: currentRoomId,
        targetUser: currentTargetUser,
        targetAvatar: currentTargetAvatar,
        isPublicRoom: currentTargetUser === "公共大厅"
    };
    localStorage.setItem('chatState', JSON.stringify(chatState));
    console.log('保存聊天状态:', chatState);
}

// 从 LocalStorage 恢复聊天状态
function restoreChatState() {
    const savedState = localStorage.getItem('chatState');
    if (savedState) {
        const chatState = JSON.parse(savedState);
        console.log('恢复聊天状态:', chatState);
        
        if (chatState.isPublicRoom) {
            joinPublicRoom(false);
        } else if (chatState.targetUser && chatState.targetUser !== "公共大厅") {
            // 立即更新标题
            updateChatHeader(chatState.targetUser);
            
            // 恢复私聊状态
            currentRoomId = chatState.roomId;
            currentTargetUser = chatState.targetUser;
            currentTargetAvatar = chatState.targetAvatar;
            
            // 连接 WebSocket
            connectWebSocket(chatState.roomId);
            
            // 更新 UI 状态
            $('.active-room').removeClass('active-room');
            $('.user-item').removeClass('active-user');
            $(`.user-item[data-user-id="${chatState.roomId}"]`).addClass('active-user');
        }
    } else {
        joinPublicRoom(false);
    }
}

function sendMessage() {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        alert('连接已断开，请刷新页面重试');
        return;
    }

    const messageInput = document.getElementById('messageInput');
    const content = messageInput.value.trim();
    if (!content) return;

    const message = {
        type: 'message',
        content: content,
        room_id: currentRoomId
    };

    ws.send(JSON.stringify(message));
    messageInput.value = '';
}

function updateChatHeader(title, onlineCount) {
    const titleElement = document.querySelector('.chat-title');
    const onlineCountElement = document.querySelector('.chat-status .online-count');
    
    if (titleElement) {
        titleElement.textContent = title;
    }
    
    if (onlineCount !== undefined && onlineCountElement) {
        onlineCountElement.textContent = onlineCount;
    }
}

function joinPublicRoom(saveState = true) {
    $('.active-room').removeClass('active-room');
    $('.user-item').removeClass('active-user');
    
    currentRoomId = "1";
    currentTargetUser = "公共大厅";
    $('#targetUser').text("公共大厅");
    $('#chatArea').show();
    
    $('.public-room').addClass('active-room');
    
    console.log('进入公共大厅，房间ID:', currentRoomId);
    connectWebSocket("1");

    if (saveState) {
        saveChatState();
    }

    updateChatHeader("公共大厅", "-");
}

function renderMessage(message) {
    const template = document.querySelector('#message-template');
    const messageElement = template.content.cloneNode(true);
    const messageDiv = messageElement.querySelector('.message');
    
    // 设置消息样式和头像
    const avatarContainer = messageDiv.querySelector('.message-avatar');
    if (message.sender_id === currentUserId) {
        messageDiv.classList.add('self');
        avatarContainer.innerHTML = currentUserAvatar ? 
            `<img src="${currentUserAvatar}" alt="我">` : 
            `<i class="fas fa-user"></i>`;
    } else {
        avatarContainer.innerHTML = currentTargetAvatar ? 
            `<img src="${currentTargetAvatar}" alt="${message.sender_name}">` : 
            `<i class="fas fa-user"></i>`;
    }
    
    // 设置发送者名称
    messageDiv.querySelector('.message-sender').textContent = 
        message.sender_id === currentUserId ? '我' : message.sender_name;
    
    // 设置消息内容
    messageDiv.querySelector('.message-content').textContent = message.content;
    
    // 设置发送时间
    const time = new Date(message.timestamp);
    messageDiv.querySelector('.message-time').textContent = 
        time.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
    
    document.querySelector('#messages').appendChild(messageElement);
    messageDiv.scrollIntoView({ behavior: 'smooth' });
}

$(document).ready(function() {
    $('#messageForm').submit(function(e) {
        e.preventDefault();
        sendMessage();
    });

    window.onbeforeunload = function() {
        if (ws) {
            ws.close();
        }
        if (wsReconnectTimer) {
            clearInterval(wsReconnectTimer);
        }
    };

    setInterval(loadUsers, 10000);
    loadUsers();
    
    // 页面加载时恢复上次的聊天状态
    restoreChatState();

    // 添加移动端菜单切换
    $(document).on('click', '.menu-toggle', function(e) {
        e.preventDefault();
        e.stopPropagation();
        console.log('菜单按钮被点击'); // 调试日志
        $('.user-list').toggleClass('active');
        $('.chat-overlay').toggleClass('active');
        $('body').toggleClass('menu-open');
    });

    // 点击遮罩层关闭菜单
    $('.chat-overlay').on('click', function() {
        closeUserList();
    });

    // ESC 键关闭菜单
    $(document).on('keyup', function(e) {
        if (e.key === "Escape") {
            closeUserList();
        }
    });
});

// 关闭用户列表
function closeUserList() {
    $('.user-list').removeClass('active');
    $('.chat-overlay').removeClass('active');
    $('body').removeClass('menu-open');
}

// 在开始新的聊天时自动关闭菜单（在移动端）
function startChat(userId, username, avatar, saveState = true) {
    $('.active-room').removeClass('active-room');
    $('.user-item').removeClass('active-user');
    
    currentRoomId = userId;
    currentTargetUser = username;
    currentTargetAvatar = avatar;
    
    $(`#user-${userId}`).addClass('active-user');
    $('#chatArea').show();
    
    console.log('开始私聊，目标用户:', username, '房间ID:', userId);
    connectWebSocket(userId);
    
    // 更新聊天标题为私聊对象的用户名
    updateChatHeader(username, "1");
    
    saveChatState();
    
    // 在移动端自动关闭菜单
    if (window.innerWidth <= 768) {
        closeUserList();
    }
}

// 在处理 WebSocket 消息时更新在线人数
function handleWebSocketMessage(event) {
    const data = JSON.parse(event.data);
    if (data.type === 'online_count') {
        document.querySelector('.chat-status .online-count').textContent = data.count;
    }
    // ... existing message handling code ...
} 