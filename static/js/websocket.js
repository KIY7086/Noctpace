let ws = null;
let wsReconnectTimer = null;

function connectWebSocket(roomId) {
    if (ws) {
        ws.close();
    }

    $('#messages').empty();
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsURL = `${protocol}//${window.location.host}/ws/${roomId}?user_id=${currentUserId}&username=${currentUsername}`;
    
    console.log('连接WebSocket:', wsURL);
    
    ws = new WebSocket(wsURL);

    ws.onopen = function() {
        console.log('WebSocket已连接 - 房间ID:', roomId);
    };

    function formatTime(timestamp) {
        const date = new Date(timestamp);
        const hours = date.getHours().toString().padStart(2, '0');
        const minutes = date.getMinutes().toString().padStart(2, '0');
        return `${hours}:${minutes}`;
    }

    ws.onmessage = function(evt) {
        const message = JSON.parse(evt.data);
        console.log('收到WebSocket消息:', message);
        
        if (message.type === 'room_info') {
            // 更新聊天标题和状态
            const chatTitle = message.username || message.content || '公共大厅';
            const isPrivateChat = message.username !== null && message.username !== '';
            
            if (isPrivateChat) {
                // 私聊时不显示人数，只显示在线状态
                const isOnline = message.online_count === 2;
                updateChatHeader(chatTitle, null, isOnline);
            } else {
                // 公共聊天显示在线人数
                updateChatHeader(chatTitle, message.online_count);
            }
        } else if (message.type === 'online_count') {
            // 只在公共聊天室更新在线人数
            const isPrivateChat = $('.chat-title').text() !== '公共大厅';
            if (!isPrivateChat) {
                updateOnlineCount(message.online_count);
            } else {
                // 私聊时更新在线状态
                const isOnline = message.online_count === 2;
                updatePrivateChatStatus(isOnline);
            }
        } else if (message.type === 'message' && message.room_id === roomId) {
            const isSelf = message.username === currentUsername;
            $('#messages').append(`
                <div class="message ${isSelf ? 'self' : ''}">
                    <div class="message-header">
                        <span class="username">${message.username}</span>
                        <span class="time">${formatTime(message.timestamp)}</span>
                    </div>
                    <div class="message-content">
                        ${message.content}
                    </div>
                </div>
            `);
            $('#messages').scrollTop($('#messages')[0].scrollHeight);
        }
    };

    ws.onerror = function(err) {
        console.error('WebSocket错误:', err);
    };

    ws.onclose = function() {
        console.log('WebSocket连接已关闭');
    };
}

// 更新聊天标题和状态
function updateChatHeader(title, onlineCount, isOnline) {
    $('.chat-title').text(title);
    
    const statusElement = $('.chat-status');
    if (onlineCount !== null && onlineCount !== undefined) {
        // 公共聊天室显示在线人数
        statusElement.html(`
            <span class="status-indicator"></span>
            <span class="online-count">${onlineCount}</span> 在线
        `);
    } else {
        // 私聊显示在线状态
        statusElement.html(`
            <span class="status-indicator ${isOnline ? 'online' : 'offline'}"></span>
            ${isOnline ? '在线' : '离线'}
        `);
    }
}

// 更新私聊在线状态
function updatePrivateChatStatus(isOnline) {
    const statusElement = $('.chat-status');
    statusElement.html(`
        <span class="status-indicator ${isOnline ? 'online' : 'offline'}"></span>
        ${isOnline ? '在线' : '离线'}
    `);
}

// 更新在线人数（仅用于公共聊天室）
function updateOnlineCount(count) {
    const isPublicRoom = $('.chat-title').text() === '公共大厅';
    if (isPublicRoom) {
        // 更新公共大厅显示
        $('.chat-status .online-count').text(count);
        const publicRoomMeta = document.querySelector('.public-room .room-meta span');
        if (publicRoomMeta) {
            publicRoomMeta.textContent = `${count} 在线`;
        }
    }
}
 