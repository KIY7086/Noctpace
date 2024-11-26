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
        
        if (message.type === 'online_count') {
            // 使用服务器发送的确切在线人数
            updateOnlineCount(message.online_count);
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

// 简化更新在线人数的函数
function updateOnlineCount(count) {
    // 更新所有显示在线人数的元素
    document.querySelectorAll('.online-count').forEach(el => {
        el.textContent = count;
    });
    
    // 更新公共大厅显示
    const publicRoomMeta = document.querySelector('.public-room .room-meta span');
    if (publicRoomMeta) {
        publicRoomMeta.textContent = `${count} 在线`;
    }
}
 