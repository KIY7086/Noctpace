let currentRoomId = null;
let currentTargetUser = null;

// 保存聊天状态到 LocalStorage
function saveChatState() {
    const chatState = {
        roomId: currentRoomId,
        targetUser: currentTargetUser,
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
        if (chatState.isPublicRoom) {
            joinPublicRoom(false);
        }
        // 如果是私聊，会在loadUsers完成后处理
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
}

function renderMessage(message) {
    const template = document.querySelector('#message-template');
    const messageElement = template.content.cloneNode(true);
    const messageDiv = messageElement.querySelector('.message');
    
    // 设置消息样式
    if (message.sender_id === currentUserId) {
        messageDiv.classList.add('self');
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
    
    // 滚动到最新消息
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
    document.querySelector('.menu-toggle')?.addEventListener('click', () => {
        document.querySelector('.user-list').classList.toggle('active');
    });

    // 页面加载完成后绑定事件
    document.addEventListener('DOMContentLoaded', function() {
        // 处理表单提交（包括回车和点击发送按钮）
        const messageForm = document.querySelector('.message-form');
        const messageInput = document.getElementById('messageInput');

        messageForm.addEventListener('submit', function(e) {
            e.preventDefault();
            sendMessage();
        });

        // 处理回车键发送
        messageInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });
    });
}); 