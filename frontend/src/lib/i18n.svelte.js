// Lightweight i18n: a reactive language store + t() lookup. Components call
// t('key') in markup; because t() reads i18n.lang ($state), toggling the
// language re-renders everything automatically. Choice persists in localStorage.

const dict = {
  ru: {
    // login
    tagline: 'Смотрите свою медиатеку вместе',
    sign_in: 'Войти',
    sign_up: 'Регистрация',
    username: 'Логин',
    password: 'Пароль',
    register_hint: 'Логин от 3 символов, пароль от 6',
    create_account: 'Создать аккаунт',
    creating: 'Создание…',
    signing_in: 'Вход…',
    err_register: 'Не удалось зарегистрироваться',
    err_credentials: 'Неверный логин или пароль',
    // header
    hdr_password: 'пароль',
    hdr_logout: 'выйти',
    // controls
    process: 'Обработать',
    reprocess: 'Переобработать',
    processing_pct: 'Обработка {pct}%',
    quality_aria: 'Качество перекодирования',
    delete_video: 'Удалить из библиотеки',
    confirm_delete: 'Удалить «{title}» из библиотеки? Файл будет удалён с диска безвозвратно.',
    // player placeholders
    ph_select: 'Выберите видео',
    ph_processing: 'Обработка {pct}%',
    ph_eta: ' · осталось ~{min} мин',
    ph_dots: '…',
    ph_error: 'Ошибка: {msg}',
    ph_error_generic: 'обработки',
    ph_press_process: 'Нажмите «Обработать»',
    // room
    party: 'Вечеринка',
    start_party: 'Начать вечеринку',
    pick_video: 'Выберите видео из библиотеки, чтобы начать',
    or: 'или',
    code: 'КОД',
    join: 'Войти',
    room_code_aria: 'Код комнаты',
    err_create_room: 'Не удалось создать комнату',
    err_room_not_found: 'Комната не найдена',
    live: 'в эфире',
    connecting: 'подключение…',
    leave: 'покинуть',
    copied: 'Скопировано',
    copy_link: 'Копировать ссылку',
    host: 'хост',
    host_controls: 'Воспроизведение ведёт хост',
    // library
    library: 'Библиотека',
    library_empty: 'Библиотека пуста',
    st_pending: 'не обработано',
    st_processing: 'обработка',
    st_ready: 'готово',
    st_error: 'ошибка',
    lib_processing_eta: 'обработка {pct}% · ~{min} мин',
    lib_processing: 'обработка {pct}%',
    // upload
    upload_video: 'Загрузить видео',
    upload_drop: 'или перетащите файлы сюда',
    up_done: 'готово',
    up_error: 'ошибка',
    // subtitle customization
    cc_settings: 'Настройки субтитров',
    cc_close: 'Закрыть',
    cc_sample: 'Пример субтитров',
    cc_drag_hint: 'Перетащите субтитры в любое место',
    cc_size: 'Размер',
    cc_text_color: 'Цвет текста',
    cc_text_opacity: 'Прозрачность текста',
    cc_bg_color: 'Фон',
    cc_bg_opacity: 'Прозрачность фона',
    cc_font: 'Шрифт',
    cc_edge: 'Обводка',
    cc_reset: 'Сбросить всё',
    cc_reset_pos: 'Сбросить позицию',
    font_sans: 'Обычный',
    font_serif: 'С засечками',
    font_mono: 'Моно',
    font_rounded: 'Округлый',
    edge_none: 'Нет',
    edge_shadow: 'Тень',
    edge_outline: 'Контур',
    edge_raised: 'Объём',
    // change password
    change_password: 'Сменить пароль',
    password_changed: 'Пароль изменён',
    current_password: 'Текущий пароль',
    new_password: 'Новый пароль',
    repeat_password: 'Повтор нового пароля',
    err_pwd_short: 'Новый пароль — минимум 6 символов',
    err_pwd_mismatch: 'Пароли не совпадают',
    err_pwd_change: 'Не удалось сменить пароль',
    cancel: 'Отмена',
    saving: 'Сохранение…',
    save: 'Сохранить',
    // server-error codes
    err_username_short: 'Имя пользователя — минимум 3 символа',
    err_password_short: 'Пароль — минимум 6 символов',
    err_username_taken: 'Имя пользователя занято',
    err_current_wrong: 'Текущий пароль неверный',
    err_too_many: 'Слишком много попыток — подождите минуту',
    // misc UI
    tap_to_sync: 'Нажмите для синхронизации',
    err_process: 'Не удалось запустить обработку',
    err_delete: 'Не удалось удалить видео',
  },
  en: {
    tagline: 'Watch your media library together',
    sign_in: 'Sign in',
    sign_up: 'Sign up',
    username: 'Username',
    password: 'Password',
    register_hint: 'Username 3+ chars, password 6+',
    create_account: 'Create account',
    creating: 'Creating…',
    signing_in: 'Signing in…',
    err_register: 'Could not register',
    err_credentials: 'Wrong username or password',
    hdr_password: 'password',
    hdr_logout: 'log out',
    process: 'Process',
    reprocess: 'Reprocess',
    processing_pct: 'Processing {pct}%',
    quality_aria: 'Transcode quality',
    delete_video: 'Remove from library',
    confirm_delete: 'Delete “{title}” from the library? The source file will be permanently removed from disk.',
    ph_select: 'Select a video',
    ph_processing: 'Processing {pct}%',
    ph_eta: ' · ~{min} min left',
    ph_dots: '…',
    ph_error: 'Error: {msg}',
    ph_error_generic: 'processing',
    ph_press_process: 'Press “Process”',
    party: 'Party',
    start_party: 'Start party',
    pick_video: 'Pick a video from the library to start',
    or: 'or',
    code: 'CODE',
    join: 'Join',
    room_code_aria: 'Room code',
    err_create_room: 'Could not create room',
    err_room_not_found: 'Room not found',
    live: 'live',
    connecting: 'connecting…',
    leave: 'leave',
    copied: 'Copied',
    copy_link: 'Copy link',
    host: 'host',
    host_controls: 'The host controls playback',
    library: 'Library',
    library_empty: 'Library is empty',
    st_pending: 'not processed',
    st_processing: 'processing',
    st_ready: 'ready',
    st_error: 'error',
    lib_processing_eta: 'processing {pct}% · ~{min} min',
    lib_processing: 'processing {pct}%',
    upload_video: 'Upload video',
    upload_drop: 'or drop files here',
    up_done: 'done',
    up_error: 'error',
    cc_settings: 'Subtitle settings',
    cc_close: 'Close',
    cc_sample: 'Subtitle preview',
    cc_drag_hint: 'Drag the subtitle anywhere',
    cc_size: 'Size',
    cc_text_color: 'Text color',
    cc_text_opacity: 'Text opacity',
    cc_bg_color: 'Background',
    cc_bg_opacity: 'Background opacity',
    cc_font: 'Font',
    cc_edge: 'Edge',
    cc_reset: 'Reset all',
    cc_reset_pos: 'Reset position',
    font_sans: 'Sans',
    font_serif: 'Serif',
    font_mono: 'Mono',
    font_rounded: 'Rounded',
    edge_none: 'None',
    edge_shadow: 'Shadow',
    edge_outline: 'Outline',
    edge_raised: 'Raised',
    change_password: 'Change password',
    password_changed: 'Password changed',
    current_password: 'Current password',
    new_password: 'New password',
    repeat_password: 'Repeat new password',
    err_pwd_short: 'New password must be at least 6 characters',
    err_pwd_mismatch: "Passwords don't match",
    err_pwd_change: 'Could not change password',
    cancel: 'Cancel',
    saving: 'Saving…',
    save: 'Save',
    err_username_short: 'Username must be at least 3 characters',
    err_password_short: 'Password must be at least 6 characters',
    err_username_taken: 'Username is taken',
    err_current_wrong: 'Current password is wrong',
    err_too_many: 'Too many attempts — wait a minute',
    tap_to_sync: 'Tap to sync',
    err_process: 'Could not start processing',
    err_delete: 'Could not delete video',
  },
}

// Maps the Go backend's stable error codes → translation keys.
const serverErrors = {
  username_short: 'err_username_short',
  password_short: 'err_password_short',
  username_taken: 'err_username_taken',
  current_wrong: 'err_current_wrong',
  invalid_credentials: 'err_credentials',
  too_many_attempts: 'err_too_many',
}

function initialLang() {
  const saved = localStorage.getItem('wp_lang')
  if (saved === 'ru' || saved === 'en') return saved
  return 'en' // English by default; users can switch to Russian
}

export const i18n = $state({ lang: initialLang() })
document.documentElement.lang = i18n.lang

export function t(key, params) {
  let s = dict[i18n.lang][key] ?? dict.en[key] ?? key
  if (params) for (const [k, v] of Object.entries(params)) s = s.replaceAll(`{${k}}`, v)
  return s
}

// Translate a backend error code when we recognize it; null for anything else
// (callers then fall back to their own generic message).
export function tCode(msg) {
  const key = serverErrors[msg]
  return key ? t(key) : null
}

export function setLang(lang) {
  i18n.lang = lang
  localStorage.setItem('wp_lang', lang)
  document.documentElement.lang = lang
}
