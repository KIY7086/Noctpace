function loadUsers() {
    $.get('/users', function(users) {
        $('#users').empty();
        users.forEach(function(user) {
            $('#users').append(
                `<div class="user-item" data-user-id="${user.id}" onclick="startChat(${user.id}, '${user.username}')">
                    ${user.username}
                </div>`
            );
        });

        // 加载用户列表后，尝试恢复聊天状态
        const savedState = localStorage.getItem('chatState');
        if (savedState) {
            const chatState = JSON.parse(savedState);
            if (!chatState.isPublicRoom) {
                const userElement = $(`.user-item:contains('${chatState.targetUser}')`);
                if (userElement.length) {
                    const userId = userElement.data('user-id');
                    startChat(userId, chatState.targetUser, false);
                }
            }
        }
    });
}

function startChat(userId, username, saveState = true) {
    $('.active-room').removeClass('active-room');
    $('.user-item').removeClass('active-user');
    
    $.post('/start-chat', {target_user_id: userId}, function(response) {
        if (response.error) {
            alert(response.error);
            return;
        }
        
        currentRoomId = response.room_id.toString();
        currentTargetUser = username;
        $('#targetUser').text(username);
        $('#chatArea').show();
        
        $(`.user-item:contains('${username}')`).addClass('active-user');
        
        console.log('开始私聊，房间ID:', currentRoomId);
        connectWebSocket(currentRoomId);

        if (saveState) {
            saveChatState();
        }
    }).fail(function(xhr) {
        alert('启动私聊失败: ' + (xhr.responseJSON?.error || '未知错误'));
    });
} 

function renderUser(user) {
    const isActive = currentChat === user.id;
    return `
        <div class="user-item ${isActive ? 'active-user' : ''}" 
             onclick="startPrivateChat('${user.id}', '${user.username}')">
            <div class="user-avatar">
                <i class="fas fa-user"></i>
            </div>
            <div class="user-info">
                <div class="user-name">${user.username}</div>
                <div class="user-status">
                    <span class="status-indicator"></span>
                    在线
                </div>
            </div>
        </div>
    `;
}

function updateUsersList(users) {
    const usersContainer = document.getElementById('users');
    usersContainer.innerHTML = users.map(renderUser).join('');
} 