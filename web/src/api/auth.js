import request from '@/utils/request'

export const login = (params) => {
  const { username: email, password } = params

  return request({
    url: '/api/admin/login',
    method: 'post',
    data: {
      email: email,
      password: password
    }
  })
}

export const logout = () => {
  return request({
    url: '/api/admin/logout',
    method: 'post'
  })
}

export const userInfo = () => {
  return request({
    url: '/api/admin/user/info',
    method: 'post'
  })
}
