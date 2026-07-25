import axios, { type AxiosRequestConfig } from 'axios'
import { API_BASE_URL, REQUEST_TIMEOUT } from '@/constants'

const instance = axios.create({
  baseURL: API_BASE_URL,
  timeout: REQUEST_TIMEOUT,
  headers: { 'Content-Type': 'application/json' }
})

instance.interceptors.response.use(
  response => {
    const res = response.data
    if (res.code !== undefined && res.code !== 0) {
      return Promise.reject(new Error(res.message || 'Request failed'))
    }
    return res.data !== undefined ? res.data : res
  },
  error => Promise.reject(error)
)

function get<T = unknown>(url: string, config?: AxiosRequestConfig) { return instance.get<T, T>(url, config) as Promise<T> }
function post<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) { return instance.post<T, T>(url, data, config) as Promise<T> }
function put<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) { return instance.put<T, T>(url, data, config) as Promise<T> }
function del<T = unknown>(url: string, config?: AxiosRequestConfig) { return instance.delete<T, T>(url, config) as Promise<T> }

export default { get, post, put, delete: del }
