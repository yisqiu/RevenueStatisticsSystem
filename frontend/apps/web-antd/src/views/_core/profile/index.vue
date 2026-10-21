<script setup lang="ts">
import { Page } from '@vben/common-ui';
import { useUserStore } from '@vben/stores';
import { Button, Card, Descriptions, DescriptionsItem } from 'ant-design-vue';
const userStore = useUserStore();
</script>

<template>
  <Page title="个人中心" description="寿县统一身份认证账号信息">
    <Card>
      <Descriptions :column="1">
        <DescriptionsItem label="用户名">{{ userStore.userInfo?.username }}</DescriptionsItem>
        <DescriptionsItem label="姓名">{{ userStore.userInfo?.realName }}</DescriptionsItem>
        <DescriptionsItem label="业务角色">{{ userStore.userInfo?.role }}</DescriptionsItem>
        <DescriptionsItem label="认证方式">寿县统一身份认证</DescriptionsItem>
      </Descriptions>
      <Button href="http://cd.libeasy.cn:8012/change-password" type="primary">修改统一认证密码</Button>
    </Card>
  </Page>
</template>
