function loadUsers() {
    $.get('/api/friends', function(users) {
        $('#users').empty();
        users.forEach(function(user) {
            const avatarHtml = user.avatar ? 
                `<img src="${user.avatar}" alt="${user.username}">` : 
                `<i class="fas fa-user"></i>`;
                
            $('#users').append(`
                <div class="user-item" data-user-id="${user.id}" onclick="startChat(${user.id}, '${user.username}', '${user.avatar}')">
                    <div class="user-avatar">
                        ${avatarHtml}
                    </div>
                    <div class="user-info">
                        <div class="user-name">${user.username}</div>
                    </div>
                </div>
            `);
        });

        // 恢复聊天状态
        const savedState = localStorage.getItem('chatState');
        if (savedState) {
            const chatState = JSON.parse(savedState);
            if (!chatState.isPublicRoom) {
                const userElement = $(`.user-item:contains('${chatState.targetUser}')`);
                if (userElement.length) {
                    const userId = userElement.data('user-id');
                    const avatar = userElement.find('.user-avatar img').attr('src');
                    startChat(userId, chatState.targetUser, avatar, false);
                }
            }
        }
    });
}

function startChat(userId, username, avatar, saveState = true) {
    $('.active-room').removeClass('active-room');
    $('.user-item').removeClass('active-user');
    
    updateChatHeader(username);
    
    $.post('/start-chat', {target_user_id: userId}, function(response) {
        if (response.error) {
            alert(response.error);
            return;
        }
        
        currentRoomId = response.room_id.toString();
        currentTargetUser = username;
        currentTargetAvatar = avatar;
        
        $('#chatArea').show();
        $(`.user-item[data-user-id="${userId}"]`).addClass('active-user');
        
        console.log('开始私聊，目标用户:', username, '房间ID:', currentRoomId);
        
        connectWebSocket(currentRoomId);

        if (saveState) {
            saveChatState();
        }
    }).fail(function(xhr) {
        alert('启动私聊失败: ' + (xhr.responseJSON?.error || '未知错误'));
    });
} 

function updateUsersList(users) {
    const usersContainer = document.getElementById('users');
    usersContainer.innerHTML = users.map(renderUser).join('');
} 