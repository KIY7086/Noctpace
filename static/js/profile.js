document.addEventListener('DOMContentLoaded', function() {
    // 加载用户信息
    loadUserProfile();

    // 绑定表单提交事件
    document.getElementById('profileForm').addEventListener('submit', function(e) {
        e.preventDefault();
        updateProfile();
    });

    // 绑定头像上传事件
    document.getElementById('avatarInput').addEventListener('change', function(e) {
        const file = e.target.files[0];
        if (file) {
            // 预览头像
            const reader = new FileReader();
            reader.onload = function(e) {
                document.getElementById('avatarPreview').src = e.target.result;
            };
            reader.readAsDataURL(file);
            
            // 上传头像
            uploadAvatar(file);
        }
    });
});

// 加载用户信息
function loadUserProfile() {
    fetch('/api/user/profile')
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            document.getElementById('emailInput').value = data.email || '';
            const avatarPreview = document.getElementById('avatarPreview');
            if (data.avatar) {
                avatarPreview.src = data.avatar;
                avatarPreview.style.display = 'block';
                avatarPreview.nextElementSibling.style.display = 'none'; // 隐藏图标
            } else {
                avatarPreview.style.display = 'none';
                avatarPreview.nextElementSibling.style.display = 'flex'; // 显示图标
            }
        })
        .catch(error => {
            console.error('加载用户信息失败:', error);
            alert('加载用户信息失败，请刷新页面重试');
        });
}

// 上传头像
function uploadAvatar(file) {
    const formData = new FormData();
    formData.append('avatar', file);

    fetch('/api/user/avatar', {
        method: 'POST',
        body: formData
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        return response.json();
    })
    .then(data => {
        alert('头像已更新');
    })
    .catch(error => {
        console.error('上传头像失败:', error);
        alert('上传头像失败，请重试');
        // 恢复默认头像
        document.getElementById('avatarPreview').src = '/static/images/default-avatar.png';
    });
}

// 更新用户信息
function updateProfile() {
    const email = document.getElementById('emailInput').value.trim();
    
    const formData = new FormData();
    formData.append('email', email);

    fetch('/api/user/profile', {
        method: 'POST',
        body: formData
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        return response.json();
    })
    .then(data => {
        alert('用户信息已更新');
    })
    .catch(error => {
        console.error('更新用户信息失败:', error);
        alert('更新用户信息失败，请重试');
    });
}

// 重置表单
function resetForm() {
    loadUserProfile();
} 