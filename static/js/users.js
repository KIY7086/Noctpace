function loadUsers() {
    $.get('/api/friends', function(users) {
        $('#users').empty();
        users.forEach(function(user) {
            const avatarHtml = user.avatar ? 
                `<img src="${user.avatar}" alt="${user.username}">` : 
                `<i class="fas fa-user"></i>`;
                
            $('#users').append(`
                <div class="user-item" data-user-id="${user.id}" data-username="${user.username}">
                    <div class="user-avatar">
                        ${avatarHtml}
                    </div>
                    <div class="user-info">
                        <div class="user-name">${user.username}</div>
                    </div>
                </div>
            `);
        });

        // 使用事件委托绑定点击事件
        $('#users').on('click', '.user-item', function() {
            const userId = $(this).data('user-id');
            const username = $(this).data('username');
            const avatar = $(this).find('img').attr('src');
            window.startChat(userId, username, avatar);
        });
    });
}

function updateUsersList(users) {
    const usersContainer = document.getElementById('users');
    usersContainer.innerHTML = users.map(renderUser).join('');
} 

function startChat(userId, username, saveState = true) {
    $('.active-room').removeClass('active-room');
    $('.user-item').removeClass('active-user');
    
    updateChatHeader(username);
    
    // 先创建/获取聊天室
    $.post('/start-chat', {target_user_id: userId}, function(response) {
        if (response.error) {
            alert(response.error);
            return;
        }
        
        currentRoomId = response.room_id.toString();
        currentTargetUser = username;
        
        $('#chatArea').show();
        $(`.user-item[data-user-id="${userId}"]`).addClass('active-user');
        
        console.log('开始私聊，目标用户:', username, '房间ID:', currentRoomId);
        
        // 使用返回的 room_id 建立 WebSocket 连接
        connectWebSocket(currentRoomId);  // 注意这里使用 currentRoomId 而不是 userId

        if (saveState) {
            saveChatState();
        }
    }).fail(function(xhr) {
        alert('启动私聊失败: ' + (xhr.responseJSON?.error || '未知错误'));
    });
}
