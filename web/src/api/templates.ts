import api from './client'
import type {
  ApiResponse,
  PaginatedResponse,
  Template,
  TemplateInput,
  TemplateExport,
  TemplatePreview,
  TemplateVersion,
  TemplateVersionInput,
  TemplateLocalization,
  TemplateLocalizationInput,
  SendTestInput,
  SendTestResponse,
} from './types'

export const templatesApi = {
  list(page = 0, size = 20, search = '') {
    return api.get<PaginatedResponse<Template>>('/users/me/templates', { params: { page, size, search } })
  },
  create(data: TemplateInput) {
    return api.post<ApiResponse<Template>>('/users/me/templates', data)
  },
  update(id: number, data: Partial<TemplateInput>) {
    return api.put<ApiResponse<Template>>(`/users/me/templates/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/users/me/templates/${id}`)
  },

  // Versions
  listVersions(templateId: number) {
    return api.get<ApiResponse<TemplateVersion[]>>(`/users/me/templates/${templateId}/versions`)
  },
  createVersion(templateId: number, data: TemplateVersionInput) {
    return api.post<ApiResponse<TemplateVersion>>(`/users/me/templates/${templateId}/versions`, data)
  },
  updateVersion(templateId: number, versionId: number, data: Partial<TemplateVersionInput>) {
    return api.put<ApiResponse<TemplateVersion>>(`/users/me/templates/${templateId}/versions/${versionId}`, data)
  },
  activateVersion(templateId: number, versionId: number) {
    return api.post<ApiResponse<Template>>(`/users/me/templates/${templateId}/activate/${versionId}`)
  },
  deleteVersion(templateId: number, versionId: number) {
    return api.delete(`/users/me/templates/${templateId}/versions/${versionId}`)
  },

  // Localizations
  listLocalizations(templateId: number, versionId: number) {
    return api.get<ApiResponse<TemplateLocalization[]>>(`/users/me/templates/${templateId}/versions/${versionId}/localizations`)
  },
  createLocalization(templateId: number, versionId: number, data: TemplateLocalizationInput) {
    return api.post<ApiResponse<TemplateLocalization>>(`/users/me/templates/${templateId}/versions/${versionId}/localizations`, data)
  },
  updateLocalization(localizationId: number, data: Partial<Omit<TemplateLocalizationInput, 'language'>>) {
    return api.put<ApiResponse<TemplateLocalization>>(`/users/me/localizations/${localizationId}`, data)
  },
  deleteLocalization(localizationId: number) {
    return api.delete(`/users/me/localizations/${localizationId}`)
  },
  previewLocalization(templateId: number, versionId: number, data: { language: string; template_data: Record<string, any> }) {
    return api.post<ApiResponse<TemplatePreview>>(`/users/me/templates/${templateId}/versions/${versionId}/preview`, data)
  },

  sendTest(templateId: number, data: SendTestInput) {
    return api.post<ApiResponse<SendTestResponse>>(`/users/me/templates/${templateId}/send-test`, data)
  },

  // Import/Export
  exportTemplate(templateId: number) {
    return api.get<ApiResponse<TemplateExport>>(`/users/me/templates/${templateId}/export`)
  },
  importTemplate(data: TemplateExport) {
    return api.post<ApiResponse<Template>>('/users/me/templates/import', data)
  },
  importHTML(file: File) {
    const formData = new FormData()
    formData.append('file', file)
    return api.post<ApiResponse<Template>>('/users/me/templates/import-html', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
}
