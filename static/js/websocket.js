let ws = null;
let wsReconnectTimer = null;

function connectWebSocket(roomId) {
    if (ws) {
        ws.close();
        clearTimeout(wsReconnectTimer);
    }

    $('#messages').empty();
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsURL = `${protocol}//${window.location.host}/ws/${roomId}?user_id=${encodeURIComponent(currentUserId)}&username=${encodeURIComponent(currentUsername)}`;
    
    console.log('连接WebSocket:', wsURL);
    
    try {
        ws = new WebSocket(wsURL);
        
        ws.onopen = function() {
            console.log('WebSocket已连接 - 房间ID:', roomId);
        };

        ws.onerror = function(err) {
            console.error('WebSocket错误:', err);
        };

        ws.onclose = function() {
            console.log('WebSocket连接已关闭，准备重连...');
            wsReconnectTimer = setTimeout(() => {
                if (currentRoomId === roomId) {
                    connectWebSocket(roomId);
                }
            }, 3000);
        };
        
        function formatTime(timestamp) {
            const date = new Date(timestamp);
            const hours = date.getHours().toString().padStart(2, '0');
            const minutes = date.getMinutes().toString().padStart(2, '0');
            return `${hours}:${minutes}`;
        }

        ws.onmessage = function(evt) {
            let message;
            try {
                message = JSON.parse(evt.data);
            } catch (e) {
                console.error('解析WebSocket消息失败:', e);
                return;
            }
            console.log('收到WebSocket消息:', message);
            
            switch(message.type) {
                case 'room_info':
                    // 更新聊天标题和状态
                    const chatTitle = message.username || message.content || '公共大厅';
                    const roomIsPrivateChat = message.username != null && message.username !== '';
                    
                    if (roomIsPrivateChat) {
                        // 私聊时不显示人数，只显示在线状态
                        const isOnline = message.online_count === 2;
                        updateChatHeader(chatTitle, null, isOnline);
                    } else {
                        // 公共聊天显示在线人数
                        updateChatHeader(chatTitle, message.online_count);
                        updateOnlineCount(message.online_count);
                    }
                    break;
                
                case 'online_count':
                    // 根据房间类型更新显示
                    if ($('.chat-title').text() === '公共大厅') {
                        updateOnlineCount(message.online_count);
                    } else {
                        const isOnline = message.online_count === 2;
                        updatePrivateChatStatus(isOnline);
                    }
                    break;
                
                case 'message':
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
                    break;
                
                case 'friend_status':
                    // 新增：处理好友状态更新
                    updateFriendStatus(message.friend_id, message.online);
                    break;
                
                case 'friend_list_update':
                    // 新增：处理好友列表更新
                    loadUsers();  // 仅在好友关系变化时才重新加载
                    break;
            }
        };
        
    } catch (e) {
        console.error('WebSocket连接失败:', e);
    }
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

// 更新在线人数的函数
function updateOnlineCount(count) {
    // 更新聊天状态栏的在线人数
    $('.chat-status').html(`
        <span class="status-indicator"></span>
        <span class="online-count">${count}</span> 在线
    `);
    
    // 更新公共大厅显示
    const publicRoomMeta = document.querySelector('.public-room .room-meta span');
    if (publicRoomMeta) {
        publicRoomMeta.textContent = `${count} 在线`;
    }
}
 