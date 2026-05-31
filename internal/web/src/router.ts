import { createRouter, createWebHistory } from 'vue-router'
import BrowseView from '@/views/BrowseView.vue'
import PlaybackView from '@/views/PlaybackView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: BrowseView },
    { path: '/clip/:id', component: PlaybackView },
  ],
})
